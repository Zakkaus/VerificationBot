package main

import (
	"context"
	"embed"
	"io/fs"
	"log"
	"os"
	"os/signal"
	"syscall"

	"verificationbot/internal/admin"
	"verificationbot/internal/bot"
	"verificationbot/internal/config"
	"verificationbot/internal/db"
)

//go:embed all:admin-ui
var adminUIEmbed embed.FS

//go:embed all:webapp
var webAppEmbed embed.FS

func main() {
	cfg := config.Load()

	// Strip the top-level directory prefix from embedded FS
	adminUI, err := fs.Sub(adminUIEmbed, "admin-ui")
	if err != nil {
		log.Fatalf("embed admin-ui: %v", err)
	}
	webApp, err := fs.Sub(webAppEmbed, "webapp")
	if err != nil {
		log.Fatalf("embed webapp: %v", err)
	}

	database, err := db.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer database.Close()

	if err := db.Migrate(database); err != nil {
		log.Fatalf("migrate db: %v", err)
	}

	if err := db.EnsureInitialAdmin(database, cfg.InitAdminUser, cfg.InitAdminPass); err != nil {
		log.Fatalf("ensure admin: %v", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Admin dashboard runs in background
	go admin.Start(ctx, database, cfg.JWTSecret, cfg.AdminHost, cfg.AdminPort, adminUI, webApp)

	// Bot runs in foreground (blocks until ctx cancelled)
	bot.Start(ctx, database, cfg)
}
