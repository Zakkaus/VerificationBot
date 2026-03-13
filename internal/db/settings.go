package db

import "database/sql"

func GetSetting(d *sql.DB, key string) string {
	var v string
	d.QueryRow("SELECT value FROM settings WHERE key=?", key).Scan(&v)
	return v
}

func SetSetting(d *sql.DB, key, value string) error {
	_, err := d.Exec("INSERT OR REPLACE INTO settings (key,value) VALUES (?,?)", key, value)
	return err
}

func GetAllSettings(d *sql.DB) (map[string]string, error) {
	rows, err := d.Query("SELECT key,value FROM settings")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		m[k] = v
	}
	return m, nil
}

func SetSettings(d *sql.DB, settings map[string]string) error {
	tx, err := d.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for k, v := range settings {
		if _, err := tx.Exec("INSERT OR REPLACE INTO settings (key,value) VALUES (?,?)", k, v); err != nil {
			return err
		}
	}
	return tx.Commit()
}
