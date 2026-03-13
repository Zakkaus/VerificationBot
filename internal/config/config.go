package config

import (
	"flag"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	TelegramToken    string
	CaptchaType      string // "recaptcha" or "turnstile"
	CaptchaSecret    string // reCAPTCHA server secret OR Turnstile secret key
	CaptchaSiteKey   string // reCAPTCHA site key OR Turnstile site key
	WebappURL        string
	ApproveMode      bool
	Ban              bool
	BanTime          int
	Shutup           bool
	TestTime         int
	Proxy            string
	Groups           []string

	// Kept for backwards compat
	RecaptchaSecret  string
	RecaptchaSiteKey string

	// Webhook (empty = polling)
	ServerHost  string
	ServerPort  int
	WebhookPath string

	// Admin dashboard
	AdminHost     string
	AdminPort     int
	JWTSecret     string
	InitAdminUser string
	InitAdminPass string

	// Database
	DBPath string
}

func Load() *Config {
	_ = godotenv.Load()

	cfg := &Config{}
	groupsCSV := flag.String("groups", env("GROUPS", ""), "Comma-separated group usernames/IDs allowlist")

	flag.StringVar(&cfg.TelegramToken, "telegram-token", env("TELEGRAM_TOKEN", ""), "Telegram bot token (required)")
	captchaSecret  := flag.String("captcha-secret",   env("CAPTCHA_SECRET",   env("RECAPTCHA_SECRET", "")),   "Captcha server secret (reCAPTCHA or Turnstile)")
	captchaSiteKey := flag.String("captcha-site-key", env("CAPTCHA_SITE_KEY", env("RECAPTCHA_SITE_KEY", "")), "Captcha site key (reCAPTCHA or Turnstile)")
	flag.StringVar(&cfg.CaptchaType, "captcha-type", env("CAPTCHA_TYPE", "recaptcha"), "Captcha type: recaptcha or turnstile")
	flag.StringVar(&cfg.WebappURL, "webapp-url", env("WEBAPP_URL", ""), "Mini App HTTPS URL (required)")

	flag.BoolVar(&cfg.ApproveMode, "approve-mode", envBool("APPROVE_MODE", true), "Use chat_join_request flow")
	flag.BoolVar(&cfg.Ban, "ban", envBool("BAN", false), "Ban users who fail verification")
	flag.IntVar(&cfg.BanTime, "ban-time", envInt("BAN_TIME", 0), "Temp ban seconds (0 = permanent)")
	flag.BoolVar(&cfg.Shutup, "shutup", envBool("SHUTUP", true), "Mute until verified (classic mode)")
	flag.IntVar(&cfg.TestTime, "test-time", envInt("TEST_TIME", 120), "Verification timeout (seconds)")
	flag.StringVar(&cfg.Proxy, "proxy", env("PROXY", ""), "Proxy URL")

	flag.StringVar(&cfg.ServerHost, "server-host", env("SERVER_HOST", ""), "Webhook bind host")
	flag.IntVar(&cfg.ServerPort, "server-port", envInt("SERVER_PORT", 0), "Webhook port")
	flag.StringVar(&cfg.WebhookPath, "webhook-path", env("WEBHOOK_PATH", "/webhook"), "Webhook path")

	flag.StringVar(&cfg.AdminHost, "admin-host", env("ADMIN_HOST", "0.0.0.0"), "Admin server bind host")
	flag.IntVar(&cfg.AdminPort, "admin-port", envInt("ADMIN_PORT", 8080), "Admin server port")
	flag.StringVar(&cfg.JWTSecret, "jwt-secret", env("JWT_SECRET", ""), "JWT signing secret (required)")
	flag.StringVar(&cfg.InitAdminUser, "admin-user", env("ADMIN_USER", "admin"), "Initial superadmin username")
	flag.StringVar(&cfg.InitAdminPass, "admin-pass", env("ADMIN_PASS", ""), "Initial superadmin password (required)")

	flag.StringVar(&cfg.DBPath, "db-path", env("DB_PATH", "bot.db"), "SQLite database path")
	flag.Parse()

	// Unify legacy reCAPTCHA flags into CaptchaSecret/CaptchaSiteKey
	cfg.CaptchaSecret  = *captchaSecret
	cfg.CaptchaSiteKey = *captchaSiteKey
	// Keep backwards-compat aliases
	cfg.RecaptchaSecret  = cfg.CaptchaSecret
	cfg.RecaptchaSiteKey = cfg.CaptchaSiteKey

	required := map[string]string{
		"--telegram-token":  cfg.TelegramToken,
		"--captcha-secret":  cfg.CaptchaSecret,
		"--captcha-site-key": cfg.CaptchaSiteKey,
		"--webapp-url":      cfg.WebappURL,
		"--jwt-secret":      cfg.JWTSecret,
		"--admin-pass":      cfg.InitAdminPass,
	}
	for flag, val := range required {
		if val == "" {
			log.Fatalf("%s is required", flag)
		}
	}

	if *groupsCSV != "" {
		for _, g := range strings.Split(*groupsCSV, ",") {
			if g = strings.TrimSpace(g); g != "" {
				cfg.Groups = append(cfg.Groups, g)
			}
		}
	}
	return cfg
}

func env(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func envBool(key string, def bool) bool {
	if v, ok := os.LookupEnv(key); ok {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}

func envInt(key string, def int) int {
	if v, ok := os.LookupEnv(key); ok {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return def
}
