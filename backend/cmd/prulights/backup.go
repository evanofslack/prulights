package main

import (
	"context"
	"flag"
	"fmt"

	"github.com/evanofslack/prulights/config"
)

// backupCmd writes a consistent copy of the database, safe while the server
// runs (a plain copy of the file can miss what's still in the WAL).
func backupCmd(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("backup", flag.ExitOnError)
	out := fs.String("out", "", "the file to write (must not exist)")
	fs.Parse(args)
	if *out == "" {
		return fmt.Errorf("backup needs -out FILE")
	}
	cfg := config.Load()
	st, err := openStore(ctx, cfg)
	if err != nil {
		return err
	}
	defer st.Close()
	if err := st.Backup(ctx, *out); err != nil {
		return err
	}
	fmt.Println("wrote", *out)
	return nil
}
