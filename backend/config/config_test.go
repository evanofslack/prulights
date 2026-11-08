package config

import (
	"slices"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	c := Load()
	if c.Port != "8080" || c.ScrapeInterval != 6*time.Hour || !c.ScrapeEnabled {
		t.Fatalf("unexpected defaults: %+v", c)
	}
	if c.RPID != "localhost" {
		t.Fatalf("RPID = %q, want localhost", c.RPID)
	}
}

func TestLoadEnv(t *testing.T) {
	t.Setenv("BASE_URL", "https://prulights.evanslack.dev/")
	t.Setenv("SCRAPE_INTERVAL", "nope")
	t.Setenv("TRUST_PROXY", "true")
	t.Setenv("METRICS_ADDR", "")
	c := Load()
	if c.BaseURL != "https://prulights.evanslack.dev" {
		t.Errorf("BaseURL = %q", c.BaseURL)
	}
	if c.RPID != "prulights.evanslack.dev" {
		t.Errorf("RPID = %q", c.RPID)
	}
	if !slices.Equal(c.RPOrigins, []string{"https://prulights.evanslack.dev"}) {
		t.Errorf("RPOrigins = %v", c.RPOrigins)
	}
	if c.ScrapeInterval != 6*time.Hour {
		t.Errorf("bad interval should keep the default, got %v", c.ScrapeInterval)
	}
	if !c.TrustProxy || c.MetricsAddr != "" {
		t.Errorf("TrustProxy=%v MetricsAddr=%q", c.TrustProxy, c.MetricsAddr)
	}
}

func TestRPOrigins(t *testing.T) {
	t.Setenv("RP_ORIGINS", "http://localhost:3000, http://localhost:8080/")
	c := Load()
	if !slices.Equal(c.RPOrigins, []string{"http://localhost:3000", "http://localhost:8080"}) {
		t.Errorf("RPOrigins = %v", c.RPOrigins)
	}
}
