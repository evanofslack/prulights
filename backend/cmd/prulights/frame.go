package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/evanofslack/prulights/config"
	"github.com/evanofslack/prulights/render"
	"github.com/evanofslack/prulights/schedule"
	"github.com/evanofslack/prulights/server"
)

// frameCmd prepares frames for flashing:
//
//	prulights frame new [-invite] [-out frame.json]
//	    on the server: make a frame's identity (token, claim code, hotspot)
//	prulights frame config -from frame.json -wifi "SSID:password" [-wifi …] -out dir
//	    anywhere, no database: the files to flash, with your Wi-Fi networks
func frameCmd(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: prulights frame new|config [flags]")
	}
	switch args[0] {
	case "new":
		return frameNew(ctx, args[1:])
	case "config":
		return frameConfig(args[1:])
	}
	return fmt.Errorf("unknown frame command %q (new or config)", args[0])
}

func frameNew(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("frame new", flag.ExitOnError)
	invite := fs.Bool("invite", true, "mark it for a new person: scanning its code makes an account and claims it")
	out := fs.String("out", "", "write the frame's identity here instead of stdout")
	fs.Parse(args)

	cfg := config.Load()
	st, err := openStore(ctx, cfg)
	if err != nil {
		return err
	}
	defer st.Close()
	fc, err := server.New(cfg, st).PrepareFrame(ctx, 0, *invite)
	if err != nil {
		return err
	}
	b, _ := json.MarshalIndent(fc, "", "  ")
	b = append(b, '\n')
	if *out == "" {
		_, err = os.Stdout.Write(b)
		return err
	}
	if err := os.WriteFile(*out, b, 0o600); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "frame #%d, code %s, written to %s (it holds the frame's token: keep it private)\n",
		fc.DeviceID, fc.ClaimCode, *out)
	return nil
}

// WifiNetwork is one network the frame may join.
type WifiNetwork struct {
	SSID     string `json:"ssid"`
	Password string `json:"password"`
}

// FlashConfig is config.json on the frame: its identity and its networks.
type FlashConfig struct {
	server.FrameConfig
	Wifi []WifiNetwork `json:"wifi"`
}

const maxNetworks = 5

func frameConfig(args []string) error {
	fs := flag.NewFlagSet("frame config", flag.ExitOnError)
	from := fs.String("from", "", "the frame's identity, from prulights frame new or the admin page")
	out := fs.String("out", "", "directory to write config.json and the first pictures into")
	var wifi urlList
	fs.Var(&wifi, "wifi", `a network, "SSID:password" (split at the first colon; repeat for more, up to 5)`)
	fs.Parse(args)
	if *from == "" || *out == "" {
		return errors.New("-from and -out are required")
	}

	raw, err := os.ReadFile(*from)
	if err != nil {
		return err
	}
	var fc server.FrameConfig
	if err := json.Unmarshal(raw, &fc); err != nil {
		return fmt.Errorf("%s: %w", *from, err)
	}
	if fc.Version != 1 || fc.Token == "" || fc.Server == "" || fc.ClaimCode == "" {
		return fmt.Errorf("%s isn't a version 1 frame identity", *from)
	}
	nets, err := parseNetworks(wifi)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(*out, 0o700); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(FlashConfig{FrameConfig: fc, Wifi: nets}, "", "  ")
	if err := os.WriteFile(filepath.Join(*out, "config.json"), append(b, '\n'), 0o600); err != nil {
		return err
	}

	// what the frame can show before it ever reaches the server
	data := render.Data{Tonight: schedule.Tonight(time.Now()), BaseURL: fc.Server,
		ClaimURL: strings.TrimRight(fc.Server, "/") + "/claim?code=" + fc.ClaimCode, ClaimCode: fc.ClaimCode,
		APSSID: fc.AP.SSID, APPassword: fc.AP.Password, PortalURL: "http://192.168.4.1"}
	for _, v := range []render.View{render.ViewSetup, render.ViewWifi} {
		f, err := render.Render(v, data)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(*out, string(v)+".bin.gz"), f.PanelBin(), 0o644); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(*out, string(v)+".png"), f.PNG(render.Measured), 0o644); err != nil {
			return err
		}
	}
	fmt.Printf("frame #%d (code %s) with %d network(s): %s/{config.json,setup.bin.gz,wifi.bin.gz}\n",
		fc.DeviceID, fc.ClaimCode, len(nets), *out)
	return nil
}

func parseNetworks(specs []string) ([]WifiNetwork, error) {
	if len(specs) > maxNetworks {
		return nil, fmt.Errorf("at most %d networks", maxNetworks)
	}
	nets := []WifiNetwork{}
	for _, spec := range specs {
		ssid, pass, ok := strings.Cut(spec, ":")
		if !ok || ssid == "" || len(ssid) > 32 {
			return nil, fmt.Errorf(`-wifi %q: want "SSID:password"`, spec)
		}
		if pass != "" && (len(pass) < 8 || len(pass) > 63) {
			return nil, fmt.Errorf("-wifi %q: a WPA password is 8 to 63 characters (empty for an open network)", ssid)
		}
		nets = append(nets, WifiNetwork{SSID: ssid, Password: pass})
	}
	return nets, nil
}
