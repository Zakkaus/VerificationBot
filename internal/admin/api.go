package admin

import (
	"encoding/csv"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"verificationbot/internal/db"
)

// --- helpers ---

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func intParam(r *http.Request, key string, def int) int {
	if s := r.URL.Query().Get(key); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			return n
		}
	}
	return def
}

// --- stats ---

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	stats, err := db.GetStats(s.db)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

// --- logs ---

func (s *Server) handleGetLogs(w http.ResponseWriter, r *http.Request) {
	f := db.LogFilter{
		Limit:  intParam(r, "limit", 50),
		Offset: intParam(r, "offset", 0),
		Result: r.URL.Query().Get("result"),
	}
	if cid, err := strconv.ParseInt(r.URL.Query().Get("chat_id"), 10, 64); err == nil {
		f.ChatID = cid
	}
	logs, err := db.GetLogs(s.db, f)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	total, _ := db.CountLogs(s.db, f)
	writeJSON(w, http.StatusOK, map[string]any{"logs": logs, "total": total})
}

func (s *Server) handleExportLogs(w http.ResponseWriter, r *http.Request) {
	logs, _ := db.GetLogs(s.db, db.LogFilter{Limit: 10000})
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="audit_logs.csv"`)
	wr := csv.NewWriter(w)
	wr.Write([]string{"ID", "Time", "Event", "UserID", "Username", "ChatID", "ChatTitle", "Detail", "Result"})
	for _, l := range logs {
		wr.Write([]string{
			strconv.FormatInt(l.ID, 10),
			l.Ts.Format(time.RFC3339),
			l.EventType,
			strconv.FormatInt(l.UserID, 10),
			l.Username,
			strconv.FormatInt(l.ChatID, 10),
			l.ChatTitle,
			l.Detail,
			l.Result,
		})
	}
	wr.Flush()
}

// --- settings (appearance only — stored in DB) ---

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := db.GetAllSettings(s.db)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (s *Server) handlePatchSettings(w http.ResponseWriter, r *http.Request) {
	var body map[string]string
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	// Only allow appearance keys (security-sensitive keys stay in env)
	allowed := map[string]bool{"site_name": true, "site_logo": true, "primary_color": true}
	filtered := make(map[string]string)
	for k, v := range body {
		if allowed[k] {
			filtered[k] = v
		}
	}
	if err := db.SetSettings(s.db, filtered); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
}

// --- users (superadmin only) ---

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := db.ListUsers(s.db)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, users)
}

func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Username == "" || body.Password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "username, password and role are required"})
		return
	}
	validRoles := map[string]bool{"superadmin": true, "admin": true, "viewer": true}
	if !validRoles[body.Role] {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "role must be superadmin, admin or viewer"})
		return
	}
	if err := db.CreateUser(s.db, body.Username, body.Password, body.Role); err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "username already exists"})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"ok": "true"})
}

func (s *Server) handlePatchUser(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	var body struct {
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	if body.Password != "" {
		db.UpdateUserPassword(s.db, id, body.Password)
	}
	if body.Role != "" {
		db.UpdateUserRole(s.db, id, body.Role)
	}
	writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
}

func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	// Prevent self-deletion
	claims := claimsFromCtx(r)
	if claims != nil && claims.UserID == id {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "cannot delete yourself"})
		return
	}
	db.DeleteUser(s.db, id)
	writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
}
