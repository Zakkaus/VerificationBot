package admin

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"verificationbot/internal/db"
)

type Claims struct {
	UserID   int64  `json:"uid"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}
	user, err := db.AuthenticateUser(s.db, body.Username, body.Password)
	if err != nil || user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}
	token, err := s.issueToken(user.ID, user.Username, user.Role, 15*time.Minute)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "token issue failed"})
		return
	}
	refresh, _ := s.issueToken(user.ID, user.Username, user.Role, 7*24*time.Hour)
	http.SetCookie(w, &http.Cookie{
		Name: "refresh", Value: refresh,
		Path:     "/admin/api/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   7 * 86400,
	})
	writeJSON(w, http.StatusOK, map[string]string{"token": token, "role": user.Role})
}

func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("refresh")
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "no refresh token"})
		return
	}
	claims, err := s.parseToken(cookie.Value)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid refresh token"})
		return
	}
	token, _ := s.issueToken(claims.UserID, claims.Username, claims.Role, 15*time.Minute)
	writeJSON(w, http.StatusOK, map[string]string{"token": token})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name: "refresh", Value: "",
		Path:    "/admin/api/",
		MaxAge:  -1,
		Expires: time.Unix(0, 0),
	})
	writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r)
	writeJSON(w, http.StatusOK, map[string]any{
		"username": claims.Username,
		"role":     claims.Role,
	})
}

func (s *Server) issueToken(userID int64, username, role string, ttl time.Duration) (string, error) {
	claims := Claims{
		UserID:   userID,
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return t.SignedString([]byte(s.jwtSecret))
}

func (s *Server) parseToken(tokenStr string) (*Claims, error) {
	t, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		return []byte(s.jwtSecret), nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := t.Claims.(*Claims); ok && t.Valid {
		return claims, nil
	}
	return nil, jwt.ErrTokenInvalidClaims
}

// handleTelegramAuth verifies a Telegram Mini App initData payload and issues a JWT.
// This is the endpoint called by the admin UI when opened as a Telegram Mini App via /admin command.
func (s *Server) handleTelegramAuth(w http.ResponseWriter, r *http.Request) {
	var body struct {
		InitData string `json:"init_data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.InitData == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "init_data required"})
		return
	}
	telegramID, ok := verifyTelegramInitData(body.InitData, s.botToken)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid initData signature"})
		return
	}
	user, err := db.GetUserByTelegramID(s.db, telegramID)
	if err != nil || user == nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "Telegram 帳號尚未綁定管理員，請請管理員在後台帳號設定中綁定你的 Telegram ID"})
		return
	}
	token, _ := s.issueToken(user.ID, user.Username, user.Role, 15*time.Minute)
	refresh, _ := s.issueToken(user.ID, user.Username, user.Role, 7*24*time.Hour)
	http.SetCookie(w, &http.Cookie{
		Name: "refresh", Value: refresh,
		Path:     "/admin/api/",
		HttpOnly: true,
		SameSite: http.SameSiteNoneMode,
		Secure:   true,
		MaxAge:   7 * 86400,
	})
	writeJSON(w, http.StatusOK, map[string]string{"token": token, "role": user.Role})
}

// verifyTelegramInitData validates a Telegram Mini App initData string.
// See: https://core.telegram.org/bots/webapps#validating-data-received-via-the-mini-app
func verifyTelegramInitData(initDataStr, botToken string) (int64, bool) {
	vals, err := url.ParseQuery(initDataStr)
	if err != nil {
		return 0, false
	}
	hash := vals.Get("hash")
	if hash == "" {
		return 0, false
	}
	vals.Del("hash")

	// Build data_check_string: sorted key=value pairs joined by \n
	keys := make([]string, 0, len(vals))
	for k := range vals {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+vals.Get(k))
	}
	dataCheckString := strings.Join(parts, "\n")

	// secret_key = HMAC-SHA256("WebAppData", bot_token)
	h1 := hmac.New(sha256.New, []byte("WebAppData"))
	h1.Write([]byte(botToken))
	secretKey := h1.Sum(nil)

	// expected = HMAC-SHA256(data_check_string, secret_key)
	h2 := hmac.New(sha256.New, secretKey)
	h2.Write([]byte(dataCheckString))
	expected := hex.EncodeToString(h2.Sum(nil))

	if !hmac.Equal([]byte(hash), []byte(expected)) {
		return 0, false
	}

	// Extract Telegram user ID from the "user" field (JSON)
	var user struct {
		ID int64 `json:"id"`
	}
	json.Unmarshal([]byte(vals.Get("user")), &user)
	return user.ID, user.ID != 0
}
