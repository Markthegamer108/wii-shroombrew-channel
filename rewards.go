package main

import (
	"context"
	"log"
)

// Downloading a title from Shroombrew is the moment the software reaches the
// console, so that is when the play reward is paid: 100 Wii Points the first
// time each title is downloaded by a wallet. A re-download or a second copy of
// the same title never pays again, so the reward cannot be farmed.
const playRewardPoints = 100

func initPlayRewards() error {
	_, err := pool.Exec(context.Background(), `
	 CREATE TABLE IF NOT EXISTS shroombrew_play_rewards (
	  wallet text NOT NULL, slug text NOT NULL,
	  at timestamptz NOT NULL DEFAULT now(),
	  PRIMARY KEY (wallet, slug));`)
	return err
}

// awardPlayPoints pays playRewardPoints once per wallet and title. It is safe to
// call on every download: the primary key makes duplicates a no-op.
func awardPlayPoints(ctx context.Context, wallet, slug string) {
	if wallet == "" || slug == "" {
		return
	}
	var inserted bool
	err := pool.QueryRow(ctx, `INSERT INTO shroombrew_play_rewards(wallet, slug)
	 VALUES($1,$2) ON CONFLICT DO NOTHING RETURNING true`, wallet, slug).Scan(&inserted)
	if err != nil || !inserted {
		return
	}
	if _, err = pool.Exec(ctx, `UPDATE shroombrew_wallets SET points = points + $2,
	 updated_at = now() WHERE id = $1`, wallet, playRewardPoints); err != nil {
		log.Printf("play reward %s: %v", slug, err)
		return
	}
	_ = ledgerAdd(ctx, pool, wallet, playRewardPoints, "play", slug)
}
