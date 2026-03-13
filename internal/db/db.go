package db

import (
	"database/sql"
	_ "modernc.org/sqlite"
)

func Open(path string) (*sql.DB, error) {
	d, err := sql.Open("sqlite", path+"?_journal_mode=WAL&_foreign_keys=on&_busy_timeout=5000")
	if err != nil {
		return nil, err
	}
	d.SetMaxOpenConns(1) // SQLite: single writer
	return d, nil
}

func Migrate(d *sql.DB) error {
	_, err := d.Exec(`
	CREATE TABLE IF NOT EXISTS admin_users (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		username      TEXT    UNIQUE NOT NULL,
		password_hash TEXT    NOT NULL,
		role          TEXT    NOT NULL CHECK(role IN ('superadmin','admin','viewer')),
		created_at    DATETIME DEFAULT CURRENT_TIMESTAMP,
		last_login    DATETIME
	);

	CREATE TABLE IF NOT EXISTS audit_logs (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		ts         DATETIME DEFAULT CURRENT_TIMESTAMP,
		event_type TEXT NOT NULL,
		user_id    INTEGER,
		username   TEXT,
		chat_id    INTEGER,
		chat_title TEXT,
		detail     TEXT,
		result     TEXT CHECK(result IN ('pass','fail','timeout','declined','pending'))
	);

	CREATE TABLE IF NOT EXISTS settings (
		key   TEXT PRIMARY KEY,
		value TEXT
	);

	CREATE INDEX IF NOT EXISTS idx_logs_ts      ON audit_logs(ts DESC);
	CREATE INDEX IF NOT EXISTS idx_logs_result  ON audit_logs(result);
	CREATE INDEX IF NOT EXISTS idx_logs_chat_id ON audit_logs(chat_id);
	CREATE INDEX IF NOT EXISTS idx_logs_user_id ON audit_logs(user_id);
	`)
	if err != nil {
		return err
	}

	// Additive migrations (safe to re-run)
	migrations := []string{
		`ALTER TABLE admin_users ADD COLUMN telegram_id INTEGER UNIQUE`,
	}
	for _, m := range migrations {
		d.Exec(m) // ignore "duplicate column" errors
	}
	d.Exec(`CREATE INDEX IF NOT EXISTS idx_users_tgid ON admin_users(telegram_id)`)


	// Insert default appearance settings (ignore if already exist)
	defaults := map[string]string{
		"site_name":     "VerificationBot Admin",
		"site_logo":     "",
		"primary_color": "#2ea6ff",
	}
	for k, v := range defaults {
		d.Exec("INSERT OR IGNORE INTO settings (key,value) VALUES (?,?)", k, v)
	}
	return nil
}
