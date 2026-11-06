package main

// Operator tools: `shroombrew admin <command>`. They talk to the database
// directly from the server shell, so there is no admin web endpoint to attack.
// Run from the install directory as the service user so config.xml and
// secrets.key are found:
//
//	cd /opt/shroombrew && sudo -u shroombrew ./shroombrew admin stats

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/jackc/pgx/v4"
)

const adminUsage = `Shroombrew admin commands:
  stats                          counts for cards, wallets, consoles, sessions, bans
  card-revoke <card number>      delete an unused card (a used card cannot be undone)
  cards-revoke-ip <ip>           delete every unused card made from that address
  ban <ip> <hours> [reason]      block an address from cards, redeeming and linking
  unban <ip>                     lift a ban
  bans                           list active bans
  wallet <friend code>           balance, consoles, sessions and the last 15 ledger lines
  adjust <friend code> <delta> <reason words...>   add or remove points (logged)
  sessions-revoke <friend code>  sign every website session out of a wallet
`

func adminMain(args []string) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" {
		fmt.Print(adminUsage)
		return 0
	}
	bg := context.Background()
	if err := adminRun(bg, args); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	return 0
}

func adminWallet(ctx context.Context, rawCode string) (string, error) {
	code := normalizeFriendCode(rawCode)
	if code == "" {
		return "", fmt.Errorf("%q is not a friend code (XXXX-XXXX-XXXX)", rawCode)
	}
	var w string
	if err := pool.QueryRow(ctx, `SELECT wallet FROM shroombrew_friends WHERE code=$1`, code).Scan(&w); err != nil {
		if err == pgx.ErrNoRows {
			return "", fmt.Errorf("no wallet has friend code %s", code)
		}
		return "", err
	}
	return w, nil
}

func adminRun(ctx context.Context, a []string) error {
	need := func(n int) error {
		if len(a) < n+1 {
			fmt.Print(adminUsage)
			return fmt.Errorf("%s needs %d argument(s)", a[0], n)
		}
		return nil
	}
	switch a[0] {
	case "stats":
		q := func(sql string) string {
			var n int64
			if err := pool.QueryRow(ctx, sql).Scan(&n); err != nil {
				return "?"
			}
			return strconv.FormatInt(n, 10)
		}
		tw := tabwriter.NewWriter(os.Stdout, 0, 8, 2, ' ', 0)
		rows := [][2]string{
			{"cards made (24h)", q(`SELECT count(*) FROM shroombrew_cards WHERE created_at > now()-interval '24 hours'`)},
			{"cards made (10 min)", q(`SELECT count(*) FROM shroombrew_cards WHERE created_at > now()-interval '10 minutes'`)},
			{"cards unused", q(`SELECT count(*) FROM shroombrew_cards WHERE redeemed_at IS NULL`)},
			{"cards redeemed (24h)", q(`SELECT count(*) FROM shroombrew_cards WHERE redeemed_at > now()-interval '24 hours'`)},
			{"cards redeemed (all kept)", q(`SELECT count(*) FROM shroombrew_cards WHERE redeemed_at IS NOT NULL`)},
			{"wrong redeem tries (24h)", q(`SELECT count(*) FROM shroombrew_attempts WHERE at > now()-interval '24 hours'`)},
			{"wallets", q(`SELECT count(*) FROM shroombrew_wallets`)},
			{"registered consoles", q(`SELECT count(*) FROM shroombrew_devices`)},
			{"points in circulation", q(`SELECT COALESCE(sum(points),0) FROM shroombrew_wallets`)},
			{"website sessions", q(`SELECT count(*) FROM shroombrew_remote`)},
			{"active bans", q(`SELECT count(*) FROM shroombrew_bans WHERE until>now()`)},
		}
		for _, r := range rows {
			fmt.Fprintf(tw, "%s\t%s\n", r[0], r[1])
		}
		return tw.Flush()
	case "card-revoke":
		if err := need(1); err != nil {
			return err
		}
		num := normalizeCard(strings.Join(a[1:], " "))
		if num == "" || !luhnValid(num) {
			return fmt.Errorf("not a valid card number")
		}
		tag, err := pool.Exec(ctx, `DELETE FROM shroombrew_cards WHERE hash=$1 AND redeemed_at IS NULL`, cardHash(num))
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			fmt.Println("nothing deleted: the card does not exist or was already redeemed")
		} else {
			fmt.Println("card deleted; it can no longer be redeemed")
		}
		return nil
	case "cards-revoke-ip":
		if err := need(1); err != nil {
			return err
		}
		tag, err := pool.Exec(ctx, `DELETE FROM shroombrew_cards WHERE creator=$1 AND redeemed_at IS NULL`, hmacHex("ip", ipBucket(a[1])))
		if err != nil {
			return err
		}
		fmt.Printf("deleted %d unused cards from %s\n", tag.RowsAffected(), ipBucket(a[1]))
		return nil
	case "ban":
		if err := need(2); err != nil {
			return err
		}
		hours, err := strconv.Atoi(a[2])
		if err != nil || hours < 1 || hours > 24*365 {
			return fmt.Errorf("hours must be 1-8760")
		}
		reason := strings.Join(a[3:], " ")
		_, err = pool.Exec(ctx, `INSERT INTO shroombrew_bans(bucket, until, reason) VALUES($1, now()+$2::int*interval '1 hour', $3)
		  ON CONFLICT (bucket) DO UPDATE SET until=EXCLUDED.until, reason=EXCLUDED.reason`, hmacHex("ip", ipBucket(a[1])), hours, reason)
		if err == nil {
			fmt.Printf("banned %s for %d hours\n", ipBucket(a[1]), hours)
		}
		return err
	case "unban":
		if err := need(1); err != nil {
			return err
		}
		tag, err := pool.Exec(ctx, `DELETE FROM shroombrew_bans WHERE bucket=$1`, hmacHex("ip", ipBucket(a[1])))
		if err == nil {
			fmt.Printf("lifted %d ban(s)\n", tag.RowsAffected())
		}
		return err
	case "bans":
		rows, err := pool.Query(ctx, `SELECT left(bucket,10), until, reason FROM shroombrew_bans WHERE until>now() ORDER BY until`)
		if err != nil {
			return err
		}
		defer rows.Close()
		n := 0
		for rows.Next() {
			var id, reason string
			var until time.Time
			if err = rows.Scan(&id, &until, &reason); err != nil {
				return err
			}
			fmt.Printf("%s…  until %s  %s\n", id, until.UTC().Format("2006-01-02 15:04Z"), reason)
			n++
		}
		if n == 0 {
			fmt.Println("no active bans (addresses are stored only as hashes)")
		}
		return rows.Err()
	case "wallet":
		if err := need(1); err != nil {
			return err
		}
		w, err := adminWallet(ctx, a[1])
		if err != nil {
			return err
		}
		points, _ := walletBalance(w)
		var devices, sessions int
		_ = pool.QueryRow(ctx, `SELECT count(*) FROM shroombrew_devices WHERE wallet=$1`, w).Scan(&devices)
		sessions = countRemote(ctx, w)
		fmt.Printf("balance %d points, %d console(s), %d website session(s)\n", points, devices, sessions)
		h, err := ledgerHistory(ctx, w, 15)
		if err != nil {
			return err
		}
		for _, l := range h {
			fmt.Printf("  %s  %+6d  -> %6d  %s %s\n", l.At, l.Delta, l.Balance, l.Reason, l.Ref)
		}
		return nil
	case "adjust":
		if err := need(3); err != nil {
			return err
		}
		delta, err := strconv.Atoi(a[2])
		if err != nil || delta == 0 || delta > 50000 || delta < -50000 {
			return fmt.Errorf("delta must be a non-zero whole number within ±50000")
		}
		w, err := adminWallet(ctx, a[1])
		if err != nil {
			return err
		}
		tx, err := pool.Begin(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback(ctx)
		tag, err := tx.Exec(ctx, `UPDATE shroombrew_wallets SET points=points+$2, updated_at=now() WHERE id=$1 AND points+$2>=0 AND points+$2<=$3`, w, delta, walletCap)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return fmt.Errorf("that would take the balance below 0 or above %d", walletCap)
		}
		if err = ledgerAdd(ctx, tx, w, delta, "admin", strings.Join(a[3:], " ")); err != nil {
			return err
		}
		if err = tx.Commit(ctx); err != nil {
			return err
		}
		p, _ := walletBalance(w)
		fmt.Printf("done; balance is now %d\n", p)
		return nil
	case "sessions-revoke":
		if err := need(1); err != nil {
			return err
		}
		w, err := adminWallet(ctx, a[1])
		if err != nil {
			return err
		}
		n, err := revokeRemote(ctx, w)
		if err == nil {
			fmt.Printf("signed out %d website session(s)\n", n)
		}
		return err
	}
	fmt.Print(adminUsage)
	return fmt.Errorf("unknown command %q", a[0])
}
