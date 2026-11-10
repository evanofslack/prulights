package main

import (
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/evanofslack/prulights/render"
	"github.com/evanofslack/prulights/server"
)

func TestFrameConfig(t *testing.T) {
	dir := t.TempDir()
	id := server.FrameConfig{Version: 1, Server: "https://prulights.example", DeviceID: 7, Token: "tok",
		ClaimCode: "K7Q-9MX", AP: server.FrameAP{SSID: "prulights-K7Q9", Password: "mxq4ad7tph"}}
	b, _ := json.Marshal(id)
	from := filepath.Join(dir, "frame.json")
	os.WriteFile(from, b, 0o600)
	out := filepath.Join(dir, "sd")

	err := frameConfig([]string{"-from", from, "-out", out, "-wifi", "Home:correct horse", "-wifi", "Cape House:pa:ss:word"})
	if err != nil {
		t.Fatal(err)
	}
	var got FlashConfig
	raw, _ := os.ReadFile(filepath.Join(out, "config.json"))
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.FrameConfig != id || len(got.Wifi) != 2 || got.Wifi[1] != (WifiNetwork{"Cape House", "pa:ss:word"}) {
		t.Errorf("config = %+v", got)
	}
	for _, name := range []string{"setup.bin.gz", "wifi.bin.gz"} {
		f, err := os.Open(filepath.Join(out, name))
		if err != nil {
			t.Fatal(err)
		}
		zr, err := gzip.NewReader(f)
		if err != nil {
			t.Fatal(err)
		}
		if raw, _ := io.ReadAll(zr); len(raw) != render.PanelBytes {
			t.Errorf("%s unpacks to %d bytes", name, len(raw))
		}
		f.Close()
	}

	for _, bad := range [][]string{
		{"-from", from, "-out", out, "-wifi", "no-colon"},
		{"-from", from, "-out", out, "-wifi", "Home:short"},
		{"-from", filepath.Join(dir, "missing.json"), "-out", out},
		{"-out", out},
	} {
		if err := frameConfig(bad); err == nil {
			t.Errorf("%v: no error", bad)
		}
	}
}
