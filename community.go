package main

// Community features: friend codes, referral rewards, gifts, update tracking,
// the points ledger, preferences and the JSON API behind the website. Every
// action is keyed by a console wallet; the website reaches a wallet only
// through a linked session (see remote.go).

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
)

const (
	referralInviterReward = 300 // points for the person who shared their code
	referralJoinerReward  = 200 // points for the person who entered it
	referralMaxRewarded   = 25  // rewarded referrals per inviter
	maxPendingGifts       = 10  // open gifts waiting on one recipient
	friendAlphabet        = "23456789ABCDEFGHJKLMNPQRSTUVWXYZ"
)

// userError is a problem the person caused and can fix. Key selects the
// translated sentence; Msg is the English fallback.
type userError struct{ Key, Msg string }

func (e *userError) Error() string { return e.Msg }

var (
	errBadCode         = &userError{"er_bad_code", "That friend code does not exist."}
	errSelfCode        = &userError{"er_self_code", "You can't use your own code."}
	errAlreadyReferred = &userError{"er_already_ref", "You already entered a friend code."}
	errInviterFull     = &userError{"er_inviter_full", "That code has reached its reward limit."}
	errUnknownApp      = &userError{"er_unknown_app", "Unknown app."}
	errAlreadyOwned    = &userError{"er_owned", "They already own that app."}
	errDupGift         = &userError{"er_dup_gift", "A gift for that app is already waiting for them."}
	errTooManyGifts    = &userError{"er_too_many_gifts", "They have too many gifts waiting."}
	errNoPoints        = &userError{"er_no_points", "Not enough Wii Points."}
	errGiftGone        = &userError{"er_gift_gone", "That gift is no longer available."}
	errNotLinked       = &userError{"er_link", "Link your Wii first."}
	errBanned          = &userError{"er_banned", "This address has been blocked for abuse."}
)

// isBanned reports whether an operator blocked this network address. Bans are
// keyed by the same hash the card tables use, so no raw address is stored.
func isBanned(ctx context.Context, ip string) bool {
	var one int
	err := pool.QueryRow(ctx, `SELECT 1 FROM shroombrew_bans WHERE bucket=$1 AND until>now()`, hmacHex("ip", ipBucket(ip))).Scan(&one)
	return err == nil
}

// waitError means "try again later" (cooldowns and lockouts).
type waitError struct {
	Key  string
	Wait time.Duration
	Msg  string
}

func (e *waitError) Error() string { return e.Msg }

func (e *waitError) seconds() int {
	s := int(e.Wait.Seconds() + 0.999)
	if s < 1 {
		s = 1
	}
	return s
}

// errText renders an error for a person in their language.
func errText(lang string, err error) string {
	var ue *userError
	var we *waitError
	switch {
	case errors.As(err, &we):
		return tl(lang, we.Key, "n", fmt.Sprint(we.seconds()), "m", fmt.Sprint((we.seconds()+59)/60))
	case errors.As(err, &ue):
		if s := tl(lang, ue.Key); s != ue.Key {
			return s
		}
		return ue.Msg
	}
	return tl(lang, "er_generic")
}

func isUserFacing(err error) bool {
	var ue *userError
	var we *waitError
	return errors.As(err, &ue) || errors.As(err, &we) || errors.Is(err, errCardBad) || errors.Is(err, errPointsCap)
}

type referralTier struct {
	Name  string
	Min   int
	Bonus int
}

var referralTiers = []referralTier{
	{"Bronze", 1, 0},
	{"Silver", 5, 500},
	{"Gold", 10, 1000},
	{"Platinum", 25, 3000},
}

// tierFor returns the highest tier reached for a referral count ("" if none).
func tierFor(count int) string {
	name := ""
	for _, t := range referralTiers {
		if count >= t.Min {
			name = t.Name
		}
	}
	return name
}

// tierBonus is the one-time bonus owed for moving from one count to another.
func tierBonus(before, after int) int {
	total := 0
	for _, t := range referralTiers {
		if before < t.Min && after >= t.Min {
			total += t.Bonus
		}
	}
	return total
}

// friendCodeFor derives a stable XXXX-XXXX-XXXX code from a wallet id.
func friendCodeFor(wallet string, salt int) string {
	h := hmacHex("friend", wallet, fmt.Sprint(salt))
	out := make([]byte, 0, 14)
	for i := 0; i < 12; i++ {
		if i > 0 && i%4 == 0 {
			out = append(out, '-')
		}
		var v byte
		fmt.Sscanf(h[i*2:i*2+2], "%02x", &v)
		out = append(out, friendAlphabet[int(v)%len(friendAlphabet)])
	}
	return string(out)
}

// normalizeFriendCode accepts any case, spaces or dashes and returns the
// canonical form, or "" when the input cannot be a code.
func normalizeFriendCode(in string) string {
	var b []byte
	for _, c := range strings.ToUpper(in) {
		switch {
		case c == '-' || c == ' ':
		case strings.ContainsRune(friendAlphabet, c):
			b = append(b, byte(c))
		default:
			return ""
		}
	}
	if len(b) != 12 {
		return ""
	}
	return string(b[0:4]) + "-" + string(b[4:8]) + "-" + string(b[8:12])
}

// seasonActive reports whether the Christmas window is open: all of December
// (UTC), when the shop plays its Christmas track and snow falls.
func seasonActive(t time.Time) (bool, string) {
	t = t.UTC()
	if t.Month() == time.December {
		return true, "christmas"
	}
	return false, ""
}

func initCommunity() error {
	_, err := pool.Exec(context.Background(), `
	 ALTER TABLE shroombrew_purchases ADD COLUMN IF NOT EXISTS version integer NOT NULL DEFAULT 0;
	 CREATE TABLE IF NOT EXISTS shroombrew_friends (
	  wallet text PRIMARY KEY REFERENCES shroombrew_wallets(id),
	  code text UNIQUE NOT NULL,
	  referred_by text REFERENCES shroombrew_wallets(id),
	  created_at timestamptz NOT NULL DEFAULT now());
	 CREATE TABLE IF NOT EXISTS shroombrew_gifts (
	  id text PRIMARY KEY,
	  from_wallet text NOT NULL REFERENCES shroombrew_wallets(id),
	  to_wallet text NOT NULL REFERENCES shroombrew_wallets(id),
	  slug text NOT NULL,
	  price integer NOT NULL CHECK(price >= 0),
	  message text NOT NULL DEFAULT '',
	  state text NOT NULL DEFAULT 'pending' CHECK(state IN ('pending','accepted','declined','cancelled')),
	  created_at timestamptz NOT NULL DEFAULT now(),
	  resolved_at timestamptz);
	 CREATE INDEX IF NOT EXISTS shroombrew_gifts_to ON shroombrew_gifts(to_wallet, state);
	 CREATE INDEX IF NOT EXISTS shroombrew_gifts_from ON shroombrew_gifts(from_wallet, state);
	 CREATE TABLE IF NOT EXISTS shroombrew_ledger (
	  id bigserial PRIMARY KEY,
	  wallet text NOT NULL REFERENCES shroombrew_wallets(id),
	  delta integer NOT NULL,
	  balance integer NOT NULL,
	  reason text NOT NULL,
	  ref text NOT NULL DEFAULT '',
	  created_at timestamptz NOT NULL DEFAULT now());
	 CREATE INDEX IF NOT EXISTS shroombrew_ledger_wallet ON shroombrew_ledger(wallet, id DESC);
	 CREATE TABLE IF NOT EXISTS shroombrew_prefs (
	  wallet text NOT NULL REFERENCES shroombrew_wallets(id),
	  key text NOT NULL,
	  value text NOT NULL,
	  PRIMARY KEY(wallet, key));
	 CREATE TABLE IF NOT EXISTS shroombrew_devices (
	  device_key text PRIMARY KEY,
	  wallet text UNIQUE NOT NULL REFERENCES shroombrew_wallets(id),
	  language text NOT NULL DEFAULT 'en',
	  region text NOT NULL DEFAULT '',
	  country text NOT NULL DEFAULT '',
	  first_seen timestamptz NOT NULL DEFAULT now(),
	  last_seen timestamptz NOT NULL DEFAULT now());
	 CREATE TABLE IF NOT EXISTS shroombrew_cards (
	  hash text PRIMARY KEY,
	  points integer NOT NULL CHECK(points > 0),
	  creator text NOT NULL,
	  created_at timestamptz NOT NULL DEFAULT now(),
	  redeemed_at timestamptz,
	  redeemed_by text REFERENCES shroombrew_wallets(id));
	 CREATE INDEX IF NOT EXISTS shroombrew_cards_creator ON shroombrew_cards(creator, created_at DESC);
	 CREATE INDEX IF NOT EXISTS shroombrew_cards_created ON shroombrew_cards(created_at);
	 CREATE TABLE IF NOT EXISTS shroombrew_attempts (
	  bucket text NOT NULL,
	  at timestamptz NOT NULL DEFAULT now());
	 CREATE INDEX IF NOT EXISTS shroombrew_attempts_bucket ON shroombrew_attempts(bucket, at DESC);
	 CREATE TABLE IF NOT EXISTS shroombrew_links (
	  code_hash text PRIMARY KEY,
	  wallet text NOT NULL REFERENCES shroombrew_wallets(id),
	  created_at timestamptz NOT NULL DEFAULT now(),
	  expires_at timestamptz NOT NULL);
	 CREATE INDEX IF NOT EXISTS shroombrew_links_wallet ON shroombrew_links(wallet);
	 CREATE TABLE IF NOT EXISTS shroombrew_remote (
	  token_hash text PRIMARY KEY,
	  wallet text NOT NULL REFERENCES shroombrew_wallets(id),
	  created_at timestamptz NOT NULL DEFAULT now(),
	  last_seen timestamptz NOT NULL DEFAULT now());
	 CREATE INDEX IF NOT EXISTS shroombrew_remote_wallet ON shroombrew_remote(wallet);
	 CREATE TABLE IF NOT EXISTS shroombrew_bans (
	  bucket text PRIMARY KEY,
	  until timestamptz NOT NULL,
	  reason text NOT NULL DEFAULT '',
	  created_at timestamptz NOT NULL DEFAULT now());`)
	return err
}

type execer interface {
	Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error)
}

// ledgerAdd records a points movement together with the balance it left.
// Call it in the same transaction as the balance change.
func ledgerAdd(ctx context.Context, q execer, wallet string, delta int, reason, ref string) error {
	_, err := q.Exec(ctx, `INSERT INTO shroombrew_ledger(wallet, delta, balance, reason, ref)
	  SELECT id, $2, points, $3, $4 FROM shroombrew_wallets WHERE id=$1`, wallet, delta, reason, ref)
	return err
}

func getPref(ctx context.Context, wallet, key string) string {
	var v string
	_ = pool.QueryRow(ctx, `SELECT value FROM shroombrew_prefs WHERE wallet=$1 AND key=$2`, wallet, key).Scan(&v)
	return v
}

func setPref(ctx context.Context, wallet, key, value string) error {
	_, err := pool.Exec(ctx, `INSERT INTO shroombrew_prefs(wallet,key,value) VALUES($1,$2,$3)
	  ON CONFLICT (wallet,key) DO UPDATE SET value=EXCLUDED.value`, wallet, key, value)
	return err
}

// ensureFriend returns the wallet's friend code, creating it on first use.
func ensureFriend(ctx context.Context, wallet string) (string, error) {
	if _, err := walletBalance(wallet); err != nil { // makes sure the wallet row exists
		return "", err
	}
	var code string
	err := pool.QueryRow(ctx, `SELECT code FROM shroombrew_friends WHERE wallet=$1`, wallet).Scan(&code)
	if err == nil {
		return code, nil
	}
	if err != pgx.ErrNoRows {
		return "", err
	}
	for salt := 0; salt < 8; salt++ {
		code = friendCodeFor(wallet, salt)
		_, err = pool.Exec(ctx, `INSERT INTO shroombrew_friends(wallet, code) VALUES($1,$2) ON CONFLICT (wallet) DO NOTHING`, wallet, code)
		if err == nil {
			// Another request may have won the race; always read back.
			if e := pool.QueryRow(ctx, `SELECT code FROM shroombrew_friends WHERE wallet=$1`, wallet).Scan(&code); e != nil {
				return "", e
			}
			return code, nil
		}
		if !isUniqueViolation(err) {
			return "", err
		}
	}
	return "", errors.New("could not allocate a friend code")
}

func isUniqueViolation(err error) bool {
	var pe *pgconn.PgError
	return errors.As(err, &pe) && pe.Code == "23505"
}

func referralCount(ctx context.Context, wallet string) (int, error) {
	var n int
	err := pool.QueryRow(ctx, `SELECT count(*) FROM shroombrew_friends WHERE referred_by=$1`, wallet).Scan(&n)
	return n, err
}

// lockWallets locks two wallet rows in a stable order so concurrent
// transactions cannot deadlock.
func lockWallets(ctx context.Context, tx pgx.Tx, a, b string) error {
	ids := []string{a, b}
	sort.Strings(ids)
	for _, id := range ids {
		var p int
		if err := tx.QueryRow(ctx, `SELECT points FROM shroombrew_wallets WHERE id=$1 FOR UPDATE`, id).Scan(&p); err != nil {
			return err
		}
	}
	return nil
}

func addPoints(ctx context.Context, tx pgx.Tx, wallet string, delta int, reason, ref string) error {
	if _, err := tx.Exec(ctx, `UPDATE shroombrew_wallets SET points=points+$2, updated_at=now() WHERE id=$1`, wallet, delta); err != nil {
		return err
	}
	return ledgerAdd(ctx, tx, wallet, delta, reason, ref)
}

// redeemReferral gives both wallets their reward the first time a wallet
// enters somebody else's friend code. Returns the points the joiner received.
func redeemReferral(ctx context.Context, wallet, rawCode string) (int, error) {
	code := normalizeFriendCode(rawCode)
	if code == "" {
		return 0, errBadCode
	}
	if _, err := ensureFriend(ctx, wallet); err != nil {
		return 0, err
	}
	var inviter string
	if err := pool.QueryRow(ctx, `SELECT wallet FROM shroombrew_friends WHERE code=$1`, code).Scan(&inviter); err != nil {
		if err == pgx.ErrNoRows {
			return 0, errBadCode
		}
		return 0, err
	}
	if inviter == wallet {
		return 0, errSelfCode
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	if err = lockWallets(ctx, tx, wallet, inviter); err != nil {
		return 0, err
	}
	var already *string
	if err = tx.QueryRow(ctx, `SELECT referred_by FROM shroombrew_friends WHERE wallet=$1`, wallet).Scan(&already); err != nil {
		return 0, err
	}
	if already != nil {
		return 0, errAlreadyReferred
	}
	var before int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM shroombrew_friends WHERE referred_by=$1`, inviter).Scan(&before); err != nil {
		return 0, err
	}
	if before >= referralMaxRewarded {
		return 0, errInviterFull
	}
	if _, err = tx.Exec(ctx, `UPDATE shroombrew_friends SET referred_by=$2 WHERE wallet=$1`, wallet, inviter); err != nil {
		return 0, err
	}
	if err = addPoints(ctx, tx, inviter, referralInviterReward, "referral", code); err != nil {
		return 0, err
	}
	if bonus := tierBonus(before, before+1); bonus > 0 {
		if err = addPoints(ctx, tx, inviter, bonus, "referral_bonus", tierFor(before+1)); err != nil {
			return 0, err
		}
	}
	if err = addPoints(ctx, tx, wallet, referralJoinerReward, "referral_join", code); err != nil {
		return 0, err
	}
	return referralJoinerReward, tx.Commit(ctx)
}

func ownsApp(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...interface{}) pgx.Row
}, wallet, slug string) (bool, error) {
	var one int
	err := q.QueryRow(ctx, `SELECT 1 FROM shroombrew_purchases WHERE wallet=$1 AND slug=$2`, wallet, slug).Scan(&one)
	if err == pgx.ErrNoRows {
		return false, nil
	}
	return err == nil, err
}

func recordPurchaseVersion(ctx context.Context, wallet, slug string, version int) {
	_, _ = pool.Exec(ctx, `UPDATE shroombrew_purchases SET version=$3 WHERE wallet=$1 AND slug=$2 AND version<$3`, wallet, slug, version)
}

// sendGift holds the sender's points until the recipient answers.
func (s *storefront) sendGift(ctx context.Context, from, toCode, slug, message string) (string, error) {
	app, ok := s.bySlug[slug]
	if !ok {
		return "", errUnknownApp
	}
	code := normalizeFriendCode(toCode)
	if code == "" {
		return "", errBadCode
	}
	var to string
	if err := pool.QueryRow(ctx, `SELECT wallet FROM shroombrew_friends WHERE code=$1`, code).Scan(&to); err != nil {
		if err == pgx.ErrNoRows {
			return "", errBadCode
		}
		return "", err
	}
	if to == from {
		return "", errSelfCode
	}
	if r := []rune(message); len(r) > 120 {
		message = string(r[:120])
	}
	price := classicPriceFor(app)
	id, err := randomHex(12)
	if err != nil {
		return "", err
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	if err = lockWallets(ctx, tx, from, to); err != nil {
		return "", err
	}
	if owned, e := ownsApp(ctx, tx, to, slug); e != nil {
		return "", e
	} else if owned {
		return "", errAlreadyOwned
	}
	var pending int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM shroombrew_gifts WHERE to_wallet=$1 AND state='pending'`, to).Scan(&pending); err != nil {
		return "", err
	}
	if pending >= maxPendingGifts {
		return "", errTooManyGifts
	}
	var dup int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM shroombrew_gifts WHERE to_wallet=$1 AND slug=$2 AND state='pending'`, to, slug).Scan(&dup); err != nil {
		return "", err
	}
	if dup > 0 {
		return "", errDupGift
	}
	res, err := tx.Exec(ctx, `UPDATE shroombrew_wallets SET points=points-$2, updated_at=now() WHERE id=$1 AND points>=$2`, from, price)
	if err != nil {
		return "", err
	}
	if res.RowsAffected() != 1 {
		return "", errNoPoints
	}
	if err = ledgerAdd(ctx, tx, from, -price, "gift_sent", slug); err != nil {
		return "", err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO shroombrew_gifts(id, from_wallet, to_wallet, slug, price, message) VALUES($1,$2,$3,$4,$5,$6)`, id, from, to, slug, price, message); err != nil {
		return "", err
	}
	return id, tx.Commit(ctx)
}

// resolveGift applies accept / decline / cancel and refunds the sender when
// the gift does not go through. Accepting something already owned refunds too.
func (s *storefront) resolveGift(ctx context.Context, wallet, id, action string) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var from, to, slug, state string
	var price int
	err = tx.QueryRow(ctx, `SELECT from_wallet, to_wallet, slug, price, state FROM shroombrew_gifts WHERE id=$1 FOR UPDATE`, id).Scan(&from, &to, &slug, &price, &state)
	if err != nil {
		if err == pgx.ErrNoRows {
			return errGiftGone
		}
		return err
	}
	if state != "pending" {
		return errGiftGone
	}
	switch action {
	case "accept", "decline":
		if wallet != to {
			return errGiftGone
		}
	case "cancel":
		if wallet != from {
			return errGiftGone
		}
	default:
		return errGiftGone
	}
	if err = lockWallets(ctx, tx, from, to); err != nil {
		return err
	}
	final := map[string]string{"accept": "accepted", "decline": "declined", "cancel": "cancelled"}[action]
	if action == "accept" {
		if owned, e := ownsApp(ctx, tx, to, slug); e != nil {
			return e
		} else if owned {
			final = "declined" // nothing to give; the sender gets the points back
		}
	}
	if final == "accepted" {
		version := 0
		if app, ok := s.bySlug[slug]; ok {
			version = app.Shop.Version
		}
		if _, err = tx.Exec(ctx, `INSERT INTO shroombrew_purchases(wallet, slug, price, version, delivered_at) VALUES($1,$2,0,$3,NULL) ON CONFLICT DO NOTHING`, to, slug, version); err != nil {
			return err
		}
	} else if err = addPoints(ctx, tx, from, price, "gift_refund", slug); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE shroombrew_gifts SET state=$2, resolved_at=now() WHERE id=$1`, id, final); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

type giftView struct {
	ID      string `json:"id"`
	Slug    string `json:"slug"`
	Name    string `json:"name"`
	Price   int    `json:"price"`
	Message string `json:"message"`
	Other   string `json:"other"` // the other side's friend code
}

func (s *storefront) listGifts(ctx context.Context, wallet string) (incoming, outgoing []giftView, err error) {
	rows, err := pool.Query(ctx, `
	 SELECT g.id, g.slug, g.price, g.message, 'in', COALESCE(f.code,'')
	   FROM shroombrew_gifts g LEFT JOIN shroombrew_friends f ON f.wallet=g.from_wallet
	  WHERE g.to_wallet=$1 AND g.state='pending'
	 UNION ALL
	 SELECT g.id, g.slug, g.price, g.message, 'out', COALESCE(f.code,'')
	   FROM shroombrew_gifts g LEFT JOIN shroombrew_friends f ON f.wallet=g.to_wallet
	  WHERE g.from_wallet=$1 AND g.state='pending'`, wallet)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var v giftView
		var dir string
		if err = rows.Scan(&v.ID, &v.Slug, &v.Price, &v.Message, &dir, &v.Other); err != nil {
			return nil, nil, err
		}
		if app, ok := s.bySlug[v.Slug]; ok {
			v.Name = app.Name
		} else {
			v.Name = v.Slug
		}
		if dir == "in" {
			incoming = append(incoming, v)
		} else {
			outgoing = append(outgoing, v)
		}
	}
	return incoming, outgoing, rows.Err()
}

type updateView struct {
	Slug      string `json:"slug"`
	Name      string `json:"name"`
	Installed int    `json:"installed"`
	Latest    int    `json:"latest"`
}

// ownedUpdates lists owned apps whose catalog version moved past the version
// recorded at purchase. Purchases made before tracking (version 0) are skipped.
func (s *storefront) ownedUpdates(ctx context.Context, wallet string) ([]updateView, error) {
	rows, err := pool.Query(ctx, `SELECT slug, version FROM shroombrew_purchases WHERE wallet=$1 AND version>0`, wallet)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []updateView
	for rows.Next() {
		var slug string
		var have int
		if err = rows.Scan(&slug, &have); err != nil {
			return nil, err
		}
		if app, ok := s.bySlug[slug]; ok && app.Shop.Version > have {
			out = append(out, updateView{Slug: slug, Name: app.Name, Installed: have, Latest: app.Shop.Version})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, rows.Err()
}

type ledgerView struct {
	Delta   int    `json:"delta"`
	Balance int    `json:"balance"`
	Reason  string `json:"reason"`
	Ref     string `json:"ref"`
	At      string `json:"at"`
}

func ledgerHistory(ctx context.Context, wallet string, limit int) ([]ledgerView, error) {
	rows, err := pool.Query(ctx, `SELECT delta, balance, reason, ref, created_at FROM shroombrew_ledger WHERE wallet=$1 ORDER BY id DESC LIMIT $2`, wallet, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ledgerView{}
	for rows.Next() {
		var v ledgerView
		var at time.Time
		if err = rows.Scan(&v.Delta, &v.Balance, &v.Reason, &v.Ref, &at); err != nil {
			return nil, err
		}
		v.At = at.UTC().Format(time.RFC3339)
		out = append(out, v)
	}
	return out, rows.Err()
}

// ---- JSON API ---------------------------------------------------------------

func jsonOut(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// jsonErr sends an error with an English message plus a key the site
// translates. Unexpected errors are logged, never shown.
func jsonErr(w http.ResponseWriter, err error) {
	var ue *userError
	var we *waitError
	switch {
	case errors.As(err, &we):
		w.Header().Set("Retry-After", fmt.Sprint(we.seconds()))
		jsonOut(w, http.StatusTooManyRequests, map[string]interface{}{"error": we.Msg, "key": we.Key, "retryAfter": we.seconds()})
	case errors.Is(err, errCardBad):
		jsonOut(w, 400, map[string]interface{}{"error": errCardBad.Error(), "key": "er_card_bad"})
	case errors.Is(err, errPointsCap):
		jsonOut(w, 400, map[string]interface{}{"error": errPointsCap.Error(), "key": "er_card_cap", "max": walletCap})
	case errors.As(err, &ue):
		status := http.StatusBadRequest
		switch ue {
		case errNoPoints:
			status = http.StatusPaymentRequired
		case errGiftGone:
			status = http.StatusNotFound
		case errNotLinked:
			status = http.StatusUnauthorized
		case errBanned:
			status = http.StatusForbidden
		}
		jsonOut(w, status, map[string]interface{}{"error": ue.Msg, "key": ue.Key})
	default:
		fmt.Printf("api error: %v\n", err)
		jsonOut(w, 500, map[string]interface{}{"error": "Something went wrong. Try again.", "key": "er_generic"})
	}
}

// A custom header cannot be set by another site without a CORS preflight, which
// this server grants only to the Pages copy of the community site (pagesCORS),
// so it doubles as CSRF protection for the JSON API.
func validAPIPost(r *http.Request) bool {
	return r.Method == http.MethodPost && r.Header.Get("X-Shroombrew") == "1"
}

func (s *storefront) communityAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	ctx := r.Context()
	route := strings.TrimPrefix(r.URL.Path, "/community/api/")
	read := func(dst interface{}) bool {
		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		if json.NewDecoder(r.Body).Decode(dst) != nil {
			jsonOut(w, 400, map[string]string{"error": "Bad request"})
			return false
		}
		return true
	}

	// Routes that need no linked Wii.
	switch {
	case route == "season" && r.Method == http.MethodGet:
		active, name := seasonActive(time.Now())
		jsonOut(w, 200, map[string]interface{}{"active": active, "name": name, "site": publicSite()})
		return
	case route == "cards/generate" && validAPIPost(r):
		var in struct {
			Points int `json:"points"`
		}
		if !read(&in) {
			return
		}
		if in.Points == 0 {
			in.Points = 1000
		}
		if !validCardPoints[in.Points] {
			jsonErr(w, fmt.Errorf("invalid card value"))
			return
		}
		num, err := generateCard(ctx, clientIP(r), in.Points)
		if err != nil {
			jsonErr(w, err)
			return
		}
		jsonOut(w, 200, map[string]interface{}{"code": formatCard(num), "points": in.Points, "cooldown": int(cardCooldown.Seconds())})
		return
	case route == "link" && validAPIPost(r):
		var in struct{ Code string }
		if !read(&in) {
			return
		}
		token, err := redeemLinkCode(ctx, in.Code, clientIP(r))
		if err != nil {
			jsonErr(w, err)
			return
		}
		if fromPages(r) {
			// The Pages site keeps the session itself and sends it as a bearer header.
			jsonOut(w, 200, map[string]interface{}{"ok": true, "token": token})
			return
		}
		setRemoteCookie(w, r, token)
		jsonOut(w, 200, map[string]bool{"ok": true})
		return
	case route == "unlink" && validAPIPost(r):
		dropRemote(ctx, r)
		clearRemoteCookie(w, r)
		jsonOut(w, 200, map[string]bool{"ok": true})
		return
	}

	wallet, linked := remoteWallet(ctx, r)
	if !linked {
		if route == "me" && r.Method == http.MethodGet {
			jsonOut(w, 200, map[string]interface{}{"linked": false, "site": publicSite()})
			return
		}
		jsonErr(w, errNotLinked)
		return
	}

	switch {
	case route == "me" && r.Method == http.MethodGet:
		code, err := ensureFriend(ctx, wallet)
		if err != nil {
			jsonErr(w, err)
			return
		}
		points, _ := walletBalance(wallet)
		n, _ := referralCount(ctx, wallet)
		in, out, _ := s.listGifts(ctx, wallet)
		ups, _ := s.ownedUpdates(ctx, wallet)
		// Empty lists must encode as [] (not null) so clients can always iterate.
		if in == nil {
			in = []giftView{}
		}
		if out == nil {
			out = []giftView{}
		}
		if ups == nil {
			ups = []updateView{}
		}
		var referred bool
		_ = pool.QueryRow(ctx, `SELECT referred_by IS NOT NULL FROM shroombrew_friends WHERE wallet=$1`, wallet).Scan(&referred)
		jsonOut(w, 200, map[string]interface{}{
			"linked": true, "code": code, "points": points, "referrals": n, "tier": tierFor(n),
			"referred": referred, "incoming": in, "outgoing": out, "updates": ups, "site": publicSite(),
			"rewards": map[string]int{"inviter": referralInviterReward, "joiner": referralJoinerReward},
		})
	case route == "history" && r.Method == http.MethodGet:
		h, err := ledgerHistory(ctx, wallet, 50)
		if err != nil {
			jsonErr(w, err)
			return
		}
		jsonOut(w, 200, map[string]interface{}{"history": h})
	case route == "updates" && r.Method == http.MethodGet:
		ups, err := s.ownedUpdates(ctx, wallet)
		if err != nil {
			jsonErr(w, err)
			return
		}
		if ups == nil {
			ups = []updateView{}
		}
		jsonOut(w, 200, map[string]interface{}{"updates": ups})
	case route == "referral" && validAPIPost(r):
		var in struct{ Code string }
		if !read(&in) {
			return
		}
		got, err := redeemReferral(ctx, wallet, in.Code)
		if err != nil {
			jsonErr(w, err)
			return
		}
		jsonOut(w, 200, map[string]int{"reward": got})
	case route == "cards/redeem" && validAPIPost(r):
		var in struct{ Code string }
		if !read(&in) {
			return
		}
		got, err := redeemCard(ctx, wallet, clientIP(r), in.Code)
		if err != nil {
			jsonErr(w, err)
			return
		}
		balance, _ := walletBalance(wallet)
		jsonOut(w, 200, map[string]int{"added": got, "points": balance})
	case route == "gift/send" && validAPIPost(r):
		var in struct{ To, Slug, Message string }
		if !read(&in) {
			return
		}
		if _, err := ensureFriend(ctx, wallet); err != nil {
			jsonErr(w, err)
			return
		}
		id, err := s.sendGift(ctx, wallet, in.To, in.Slug, in.Message)
		if err != nil {
			jsonErr(w, err)
			return
		}
		jsonOut(w, 200, map[string]string{"id": id})
	case (route == "gift/accept" || route == "gift/decline" || route == "gift/cancel") && validAPIPost(r):
		var in struct{ ID string }
		if !read(&in) {
			return
		}
		if err := s.resolveGift(ctx, wallet, in.ID, strings.TrimPrefix(route, "gift/")); err != nil {
			jsonErr(w, err)
			return
		}
		jsonOut(w, 200, map[string]bool{"ok": true})
	default:
		w.Header().Set("Allow", "GET, POST")
		jsonOut(w, 404, map[string]string{"error": "Not found"})
	}
}

// ---- seasonal script --------------------------------------------------------

// seasonScript serves shop/fx.js (music, December snow, reversed-text day and
// their settings) year round. The community pages on GitHub Pages load it as
// /shroombrew-season.js, the web shop as /shroombrew-fx.js; the script itself
// checks the date.
func (s *storefront) seasonScript(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	data, err := shopFiles.ReadFile("shop/fx.js")
	if err != nil {
		_, _ = w.Write([]byte("/* season script missing */\n"))
		return
	}
	_, _ = w.Write(data)
}
