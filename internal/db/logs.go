package db

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type AuditLog struct {
	ID        int64
	Ts        time.Time
	EventType string
	UserID    int64
	Username  string
	ChatID    int64
	ChatTitle string
	Detail    string
	Result    string
}

type LogFilter struct {
	Limit     int
	Offset    int
	ChatID    int64
	UserID    int64
	Result    string
	Search    string     // fuzzy: username, event_type, detail, chat_title
	StartTime *time.Time
	EndTime   *time.Time
}


func AddLog(d *sql.DB, l AuditLog) error {
	_, err := d.Exec(
		`INSERT INTO audit_logs (event_type, user_id, username, chat_id, chat_title, detail, result)
		 VALUES (?,?,?,?,?,?,?)`,
		l.EventType, l.UserID, l.Username, l.ChatID, l.ChatTitle, l.Detail, l.Result,
	)
	return err
}

func buildLogWhere(f LogFilter) (string, []any) {
	var conds []string
	var args []any
	if f.ChatID != 0 {
		conds = append(conds, "chat_id = ?")
		args = append(args, f.ChatID)
	}
	if f.UserID != 0 {
		conds = append(conds, "user_id = ?")
		args = append(args, f.UserID)
	}
	if f.Result != "" {
		conds = append(conds, "result = ?")
		args = append(args, f.Result)
	}
	if f.Search != "" {
		like := "%" + f.Search + "%"
		conds = append(conds, "(username LIKE ? OR event_type LIKE ? OR detail LIKE ? OR chat_title LIKE ?)")
		args = append(args, like, like, like, like)
	}
	if f.StartTime != nil {
		conds = append(conds, "ts >= ?")
		args = append(args, f.StartTime.Format(time.RFC3339))
	}
	if f.EndTime != nil {
		conds = append(conds, "ts <= ?")
		args = append(args, f.EndTime.Format(time.RFC3339))
	}
	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}
	return where, args
}

func scanLog(rows *sql.Rows) (AuditLog, error) {
	var l AuditLog
	var ts string
	err := rows.Scan(&l.ID, &ts, &l.EventType, &l.UserID, &l.Username,
		&l.ChatID, &l.ChatTitle, &l.Detail, &l.Result)
	if err == nil {
		l.Ts, _ = time.Parse("2006-01-02 15:04:05", ts)
	}
	return l, err
}

func GetLogs(d *sql.DB, f LogFilter) ([]AuditLog, error) {
	where, args := buildLogWhere(f)
	q := fmt.Sprintf(
		"SELECT id,ts,event_type,user_id,username,chat_id,chat_title,detail,result FROM audit_logs %s ORDER BY ts DESC LIMIT ? OFFSET ?",
		where)
	if f.Limit <= 0 {
		f.Limit = 50
	}
	args = append(args, f.Limit, f.Offset)

	rows, err := d.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []AuditLog
	for rows.Next() {
		l, err := scanLog(rows)
		if err != nil {
			return nil, err
		}
		logs = append(logs, l)
	}
	return logs, nil
}

func CountLogs(d *sql.DB, f LogFilter) (int, error) {
	where, args := buildLogWhere(f)
	q := fmt.Sprintf("SELECT COUNT(*) FROM audit_logs %s", where)
	var n int
	err := d.QueryRow(q, args...).Scan(&n)
	return n, err
}

func GetStats(d *sql.DB) (map[string]int, error) {
	stats := map[string]int{"pass": 0, "fail": 0, "timeout": 0, "declined": 0, "total": 0}
	rows, err := d.Query(
		`SELECT result, COUNT(*) FROM audit_logs
		 WHERE ts >= datetime('now','-24 hours') GROUP BY result`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var result string
		var count int
		if err := rows.Scan(&result, &count); err != nil {
			return nil, err
		}
		stats[result] = count
		stats["total"] += count
	}
	return stats, nil
}
