package db

import (
	"database/sql"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type AdminUser struct {
	ID         int64
	Username   string
	Role       string
	TelegramID int64
	CreatedAt  time.Time
	LastLogin  *time.Time
}

func EnsureInitialAdmin(d *sql.DB, username, password string) error {
	var count int
	if err := d.QueryRow("SELECT COUNT(*) FROM admin_users").Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return err
	}
	_, err = d.Exec(
		"INSERT INTO admin_users (username,password_hash,role) VALUES (?,?,'superadmin')",
		username, string(hash),
	)
	return err
}

func AuthenticateUser(d *sql.DB, username, password string) (*AdminUser, error) {
	var hash string
	u := &AdminUser{}
	var lastLoginStr sql.NullString
	var createdStr string
	err := d.QueryRow(
		"SELECT id,username,password_hash,role,created_at,last_login FROM admin_users WHERE username=?",
		username,
	).Scan(&u.ID, &u.Username, &hash, &u.Role, &createdStr, &lastLoginStr)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return nil, nil
	}
	d.Exec("UPDATE admin_users SET last_login=CURRENT_TIMESTAMP WHERE id=?", u.ID)
	return u, nil
}

func ListUsers(d *sql.DB) ([]AdminUser, error) {
	rows, err := d.Query(
		"SELECT id,username,role,telegram_id,created_at,last_login FROM admin_users ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []AdminUser
	for rows.Next() {
		u := AdminUser{}
		var lastStr sql.NullString
		var createdStr string
		var tgID sql.NullInt64
		if err := rows.Scan(&u.ID, &u.Username, &u.Role, &tgID, &createdStr, &lastStr); err != nil {
			return nil, err
		}
		u.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdStr)
		if lastStr.Valid {
			t, _ := time.Parse("2006-01-02 15:04:05", lastStr.String)
			u.LastLogin = &t
		}
		if tgID.Valid {
			u.TelegramID = tgID.Int64
		}
		users = append(users, u)
	}
	return users, nil
}

// GetUserByTelegramID looks up an admin user by their linked Telegram user ID.
func GetUserByTelegramID(d *sql.DB, telegramID int64) (*AdminUser, error) {
	u := &AdminUser{}
	var lastStr sql.NullString
	var createdStr string
	err := d.QueryRow(
		"SELECT id,username,role,telegram_id,created_at,last_login FROM admin_users WHERE telegram_id=?",
		telegramID,
	).Scan(&u.ID, &u.Username, &u.Role, &u.TelegramID, &createdStr, &lastStr)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	u.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdStr)
	if lastStr.Valid {
		t, _ := time.Parse("2006-01-02 15:04:05", lastStr.String)
		u.LastLogin = &t
	}
	return u, nil
}

// SetUserTelegramID links (or unlinks) a Telegram user ID to an admin account.
func SetUserTelegramID(d *sql.DB, adminUserID, telegramID int64) error {
	var val any = telegramID
	if telegramID == 0 {
		val = nil // unlink
	}
	_, err := d.Exec("UPDATE admin_users SET telegram_id=? WHERE id=?", val, adminUserID)
	return err
}

func CreateUser(d *sql.DB, username, password, role string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return err
	}
	_, err = d.Exec(
		"INSERT INTO admin_users (username,password_hash,role) VALUES (?,?,?)",
		username, string(hash), role,
	)
	return err
}

func UpdateUserPassword(d *sql.DB, id int64, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return err
	}
	_, err = d.Exec("UPDATE admin_users SET password_hash=? WHERE id=?", string(hash), id)
	return err
}

func UpdateUserRole(d *sql.DB, id int64, role string) error {
	_, err := d.Exec("UPDATE admin_users SET role=? WHERE id=?", role, id)
	return err
}

func DeleteUser(d *sql.DB, id int64) error {
	_, err := d.Exec("DELETE FROM admin_users WHERE id=?", id)
	return err
}
