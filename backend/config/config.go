// Package config reads the server's settings from the environment.
package config

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config is everything the server reads from the environment. A value that
// doesn't parse is logged and the default kept.
type Config struct {
	Port         string // PORT
	StaticDir    string // STATIC_DIR: the web app's static export
	DatabasePath string // DATABASE_PATH: the sqlite file
	MetricsAddr  string // METRICS_ADDR: where /metrics is served, empty for off; never the public port
	BaseURL      string // BASE_URL: the public origin, e.g. https://prulights.evanslack.dev
	TrustProxy   bool   // TRUST_PROXY: take the client address from CF-Connecting-IP / X-Forwarded-For
	Dev          bool   // DEV: dev-only routes and relaxed origins
	Version      string // set by main, not the environment

	PublicRateLimit int // PUBLIC_RATE_LIMIT: public api requests per minute per client

	ScrapeEnabled  bool          // SCRAPE_ENABLED
	ScrapeInterval time.Duration // SCRAPE_INTERVAL

	RPID      string   // RP_ID: the WebAuthn relying party id, the host of BASE_URL by default
	RPOrigins []string // RP_ORIGINS: comma separated origins allowed to use passkeys, BASE_URL by default
}

// Default is the config with nothing set.
func Default() Config {
	return Config{
		Port:           "8080",
		StaticDir:      "../web/out",
		DatabasePath:   "prulights.db",
		MetricsAddr:    ":2112",
		BaseURL:        "http://localhost:8080",
		ScrapeEnabled:  true,
		ScrapeInterval: 6 * time.Hour,

		PublicRateLimit: 60,
	}
}

// Load reads the environment over the defaults.
func Load() Config {
	c := Default()
	envString("PORT", &c.Port)
	envString("STATIC_DIR", &c.StaticDir)
	envString("DATABASE_PATH", &c.DatabasePath)
	if s, ok := os.LookupEnv("METRICS_ADDR"); ok {
		c.MetricsAddr = s
	}
	envString("BASE_URL", &c.BaseURL)
	c.BaseURL = strings.TrimRight(c.BaseURL, "/")
	envBool("TRUST_PROXY", &c.TrustProxy)
	envBool("DEV", &c.Dev)
	envInt("PUBLIC_RATE_LIMIT", &c.PublicRateLimit)
	envBool("SCRAPE_ENABLED", &c.ScrapeEnabled)
	envDuration("SCRAPE_INTERVAL", &c.ScrapeInterval)
	envString("RP_ID", &c.RPID)
	if s := os.Getenv("RP_ORIGINS"); s != "" {
		for _, o := range strings.Split(s, ",") {
			if o = strings.TrimSpace(o); o != "" {
				c.RPOrigins = append(c.RPOrigins, strings.TrimRight(o, "/"))
			}
		}
	}
	c.Fill()
	return c
}

// Fill derives the passkey settings from BASE_URL when they aren't set.
func (c *Config) Fill() {
	if c.RPID == "" {
		host := c.BaseURL
		if i := strings.Index(host, "://"); i >= 0 {
			host = host[i+3:]
		}
		if i := strings.IndexAny(host, ":/"); i >= 0 {
			host = host[:i]
		}
		c.RPID = host
	}
	if len(c.RPOrigins) == 0 {
		c.RPOrigins = []string{c.BaseURL}
	}
}

func envString(key string, v *string) {
	if s := os.Getenv(key); s != "" {
		*v = s
	}
}

func envInt(key string, v *int) {
	if s := os.Getenv(key); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			*v = n
		} else {
			slog.Warn("invalid config, using default", "key", key, "value", s, "default", *v)
		}
	}
}

func envDuration(key string, v *time.Duration) {
	if s := os.Getenv(key); s != "" {
		if d, err := time.ParseDuration(s); err == nil && d > 0 {
			*v = d
		} else {
			slog.Warn("invalid config, using default", "key", key, "value", s, "default", *v)
		}
	}
}

func envBool(key string, v *bool) {
	if s := os.Getenv(key); s != "" {
		if b, err := strconv.ParseBool(s); err == nil {
			*v = b
		} else {
			slog.Warn("invalid config, using default", "key", key, "value", s, "default", *v)
		}
	}
}
