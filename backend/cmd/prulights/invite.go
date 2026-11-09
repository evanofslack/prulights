package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/evanofslack/prulights/config"
	"github.com/evanofslack/prulights/server"
	"github.com/evanofslack/prulights/store"
	"github.com/mdp/qrterminal/v3"
)

// inviteCmd makes an invite link and prints it with a QR code to scan. With
// -for-user it's a re-invite: the link adds a passkey to that person, keeping
// everything they've logged.
func inviteCmd(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("invite", flag.ExitOnError)
	admin := fs.Bool("admin", false, "the new person is an admin")
	forUser := fs.String("for-user", "", "add a passkey for this person instead of inviting someone new")
	noQR := fs.Bool("no-qr", false, "print only the link")
	fs.Parse(args)

	cfg := config.Load()
	st, err := openStore(ctx, cfg)
	if err != nil {
		return err
	}
	defer st.Close()

	now := time.Now()
	inv := store.Invite{IsAdmin: *admin, CreatedAt: now, ExpiresAt: now.Add(server.InviteTTL)}
	who := "someone new"
	if *forUser != "" {
		u, err := st.UserByName(ctx, *forUser)
		if errors.Is(err, store.ErrNotFound) {
			return fmt.Errorf("nobody is named %q", *forUser)
		}
		if err != nil {
			return err
		}
		inv.ForUser, who = &u.ID, u.Name+" (adds a passkey)"
	}
	token, err := st.CreateInvite(ctx, inv)
	if err != nil {
		return err
	}
	url := server.InviteURL(cfg.BaseURL, token)
	if !*noQR {
		qrterminal.GenerateHalfBlock(url, qrterminal.L, os.Stdout)
	}
	fmt.Printf("invite for %s, good until %s:\n%s\n", who, inv.ExpiresAt.Format("Mon Jan 2 3:04pm"), url)
	return nil
}
