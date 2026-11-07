package main

// Cards are minted on the website and stored as a keyed hash, so a leak of
// the database gives away nothing usable. Any other 16-digit Luhn number
// redeems as a standard 1000-point card, as on the old WiiMart revival.

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/jackc/pgx/v4"
)

const (
	cardPoints         = 1000
	cardCooldown       = 15 * time.Second
	cardMaxOutstanding = 25
	cardDailyCap       = 300
	cardGlobalPer10Min = 600
	cardTTL            = "30 days"
	cardUsedKeep       = "90 days"
	walletCap          = 50000
	redeemFailLimit    = 8
	redeemFailWindow   = "10 minutes"
)

var (
	errCardBad   = errors.New("That card number is not valid or was already used.")
	errPointsCap = errors.New("Your balance is full.")
)

// luhnCheckDigit returns the check digit that completes a 15-digit number.
func luhnCheckDigit(d15 string) byte {
	sum := 0
	for i := 0; i < len(d15); i++ {
		d := int(d15[len(d15)-1-i] - '0')
		if i%2 == 0 {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += d
	}
	return byte('0' + (10-sum%10)%10)
}

func luhnValid(d16 string) bool {
	if len(d16) != 16 {
		return false
	}
	for _, c := range d16 {
		if c < '0' || c > '9' {
			return false
		}
	}
	return luhnCheckDigit(d16[:15]) == d16[15]
}

func newCardNumber() (string, error) {
	var b strings.Builder
	ten := big.NewInt(10)
	for i := 0; i < 15; i++ {
		n, err := rand.Int(rand.Reader, ten)
		if err != nil {
			return "", err
		}
		b.WriteByte(byte('0' + n.Int64()))
	}
	d := b.String()
	return d + string(luhnCheckDigit(d)), nil
}

// normalizeCard keeps only the 16 digits; spaces and dashes are allowed as
// separators, anything else makes the input invalid.
func normalizeCard(raw string) string {
	var b strings.Builder
	for _, c := range raw {
		switch {
		case c >= '0' && c <= '9':
			b.WriteRune(c)
		case c == ' ' || c == '-':
		default:
			return ""
		}
	}
	if b.Len() != 16 {
		return ""
	}
	return b.String()
}

func formatCard(n string) string {
	return n[0:4] + " " + n[4:8] + " " + n[8:12] + " " + n[12:16]
}

func cardHash(num string) string { return hmacHex("card", num) }

// Card values you can mint.
var validCardPoints = map[int]bool{1000: true, 2000: true, 3000: true, 4000: true, 5000: true}

// Mint a card worth points. The value is stored so redeeming pays exactly that much.
func generateCard(ctx context.Context, ip string, points int) (string, error) {
	if !validCardPoints[points] {
		return "", fmt.Errorf("invalid card value")
	}
	if isBanned(ctx, ip) {
		return "", errBanned
	}
	creator := hmacHex("ip", ipBucket(ip))
	tx, err := pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	// Serialise concurrent requests from the same address so the cooldown
	// cannot be beaten by firing requests in parallel.
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, creator); err != nil {
		return "", err
	}
	var sinceLast float64
	if err = tx.QueryRow(ctx, `SELECT COALESCE(EXTRACT(EPOCH FROM (now() - max(created_at))), 1e9) FROM shroombrew_cards WHERE creator=$1`, creator).Scan(&sinceLast); err != nil {
		return "", err
	}
	if wait := cardCooldown - time.Duration(sinceLast*float64(time.Second)); wait > 0 {
		return "", &waitError{Key: "er_wait", Wait: wait, Msg: "Please wait before making another card."}
	}
	var outstanding, today, recent int
	if err = tx.QueryRow(ctx, `SELECT
	   count(*) FILTER (WHERE redeemed_at IS NULL AND created_at > now() - interval '`+cardTTL+`'),
	   count(*) FILTER (WHERE created_at > now() - interval '24 hours')
	  FROM shroombrew_cards WHERE creator=$1`, creator).Scan(&outstanding, &today); err != nil {
		return "", err
	}
	if outstanding >= cardMaxOutstanding {
		return "", &waitError{Key: "er_card_limit", Wait: time.Minute, Msg: "You already have unused card numbers. Use them first."}
	}
	if today >= cardDailyCap {
		return "", &waitError{Key: "er_card_daily", Wait: time.Hour, Msg: "Daily limit reached. Come back tomorrow."}
	}
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM shroombrew_cards WHERE created_at > now() - interval '10 minutes'`).Scan(&recent); err != nil {
		return "", err
	}
	if recent >= cardGlobalPer10Min {
		return "", &waitError{Key: "er_busy", Wait: time.Minute, Msg: "The card machine is busy. Try again in a minute."}
	}
	for attempt := 0; attempt < 5; attempt++ {
		num, err := newCardNumber()
		if err != nil {
			return "", err
		}
		tag, err := tx.Exec(ctx, `INSERT INTO shroombrew_cards(hash, points, creator) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, cardHash(num), points, creator)
		if err != nil {
			return "", err
		}
		if tag.RowsAffected() == 1 {
			return num, tx.Commit(ctx)
		}
	}
	return "", errors.New("could not allocate a card number")
}

// lockoutWait returns how long a bucket stays locked after too many wrong tries.
func lockoutWait(ctx context.Context, bucket string, limit int) (time.Duration, error) {
	var n int
	var secs float64
	err := pool.QueryRow(ctx, `SELECT count(*), COALESCE(EXTRACT(EPOCH FROM (min(at) + interval '`+redeemFailWindow+`' - now())), 0)
	  FROM (SELECT at FROM shroombrew_attempts WHERE bucket=$1 AND at > now() - interval '`+redeemFailWindow+`' ORDER BY at DESC LIMIT $2) t`, bucket, limit).Scan(&n, &secs)
	if err != nil {
		return 0, err
	}
	if n >= limit && secs > 0 {
		return time.Duration(secs * float64(time.Second)), nil
	}
	return 0, nil
}

func recordFail(ctx context.Context, buckets ...string) {
	for _, b := range buckets {
		_, _ = pool.Exec(ctx, `INSERT INTO shroombrew_attempts(bucket) VALUES($1)`, b)
	}
}

// cardValue is what a card number is worth: a minted card's points while it
// is unused, otherwise the standard card (see redeemCard).
func cardValue(ctx context.Context, num string) int {
	var points int
	if pool.QueryRow(ctx, `SELECT points FROM shroombrew_cards WHERE hash=$1 AND redeemed_at IS NULL AND created_at > now() - interval '`+cardTTL+`'`, cardHash(num)).Scan(&points) == nil {
		return points
	}
	return cardPoints
}

// redeemCard turns a card number into points. A card minted on the website
// gives its points once. Wii Points are free, so, as on the WiiMart revival,
// any other 16-digit number is a standard 1000-point card; only input that is
// not a card number counts toward the lockout.
func redeemCard(ctx context.Context, wallet, ip, raw string) (int, error) {
	if isBanned(ctx, ip) {
		return 0, errBanned
	}
	bw, bi := hmacHex("lock-wallet", wallet), hmacHex("lock-ip", ipBucket(ip))
	for _, b := range []string{bw, bi} {
		wait, err := lockoutWait(ctx, b, redeemFailLimit)
		if err != nil {
			return 0, err
		}
		if wait > 0 {
			return 0, &waitError{Key: "er_locked", Wait: wait, Msg: "Too many wrong tries. Try again later."}
		}
	}
	num := normalizeCard(raw)
	if num == "" {
		recordFail(ctx, bw, bi)
		return 0, errCardBad
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	var balance int
	if err = tx.QueryRow(ctx, `SELECT points FROM shroombrew_wallets WHERE id=$1 FOR UPDATE`, wallet).Scan(&balance); err != nil {
		return 0, err
	}
	points, ref := cardPoints, "standard"
	var minted int
	err = tx.QueryRow(ctx, `UPDATE shroombrew_cards SET redeemed_at=now(), redeemed_by=$2
	   WHERE hash=$1 AND redeemed_at IS NULL AND created_at > now() - interval '`+cardTTL+`' RETURNING points`, cardHash(num), wallet).Scan(&minted)
	switch {
	case err == nil:
		points, ref = minted, cardHash(num)[:12]
	case err != pgx.ErrNoRows:
		return 0, err
	}
	if balance+points > walletCap {
		return 0, errPointsCap // rolled back: a minted card stays unused
	}
	if err = addPoints(ctx, tx, wallet, points, "card", ref); err != nil {
		return 0, err
	}
	return points, tx.Commit(ctx)
}

// cardJanitor removes expired and old rows so the tables cannot grow forever.
func cardJanitor() {
	for {
		c := context.Background()
		_, _ = pool.Exec(c, `DELETE FROM shroombrew_cards WHERE (redeemed_at IS NULL AND created_at < now() - interval '`+cardTTL+`') OR (redeemed_at IS NOT NULL AND redeemed_at < now() - interval '`+cardUsedKeep+`')`)
		_, _ = pool.Exec(c, `DELETE FROM shroombrew_attempts WHERE at < now() - interval '1 day'`)
		_, _ = pool.Exec(c, `DELETE FROM shroombrew_links WHERE expires_at < now()`)
		_, _ = pool.Exec(c, `DELETE FROM shroombrew_remote WHERE last_seen < now() - interval '90 days'`)
		time.Sleep(time.Hour)
	}
}
