package admin

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
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

// handleTelegramAuth verifies Telegram Mini App initData and checks if user is a group admin.
// No password or DB account needed — group admin status = access.
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
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "initData 簽名無效"})
		return
	}

	// Check group admin status via Telegram API
	role := checkGroupAdminRole(s.botToken, s.groups, telegramID)
	if role == "" {
		writeJSON(w, http.StatusForbidden, map[string]string{
			"error": "🚫 你不是任何配置群組的管理員，無法訪問管理後台",
		})
		return
	}

	// Use Telegram user ID as the username in JWT (no DB account needed)
	username := fmt.Sprintf("tg:%d", telegramID)
	token, _ := s.issueToken(telegramID, username, role, 2*time.Hour)
	refresh, _ := s.issueToken(telegramID, username, role, 7*24*time.Hour)
	http.SetCookie(w, &http.Cookie{
		Name: "refresh", Value: refresh,
		Path:     "/admin/api/",
		HttpOnly: true,
		SameSite: http.SameSiteNoneMode,
		Secure:   true,
		MaxAge:   7 * 86400,
	})
	writeJSON(w, http.StatusOK, map[string]string{"token": token, "role": role})
}

// checkGroupAdminRole calls Telegram getChatMember for each group.
// Returns "superadmin" (creator), "admin" (group admin), or "" (not an admin).
// Tries multiple ID formats to handle regular/supergroup IDs.
func checkGroupAdminRole(botToken string, groups []string, userID int64) string {
	result := ""
	for _, group := range groups {
		// Build candidate chat_id strings to try (handle 2452654588 / -2452654588 / -1002452654588)
		toTry := []string{group}
		if id, err := strconv.ParseInt(strings.TrimPrefix(group, "@"), 10, 64); err == nil {
			abs := id
			if abs < 0 { abs = -abs }
			absStr := strconv.FormatInt(abs, 10)
			if strings.HasPrefix(absStr, "100") { absStr = absStr[3:] }
			toTry = []string{
				absStr,
				"-" + absStr,
				"-100" + absStr,
			}
		}
		for _, cid := range toTry {
			url := fmt.Sprintf("https://api.telegram.org/bot%s/getChatMember?chat_id=%s&user_id=%d",
				botToken, cid, userID)
			resp, err := http.Get(url) //nolint:noctx
			if err != nil { continue }
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			var res struct {
				OK     bool `json:"ok"`
				Result struct {
					Status string `json:"status"`
				} `json:"result"`
			}
			if err := json.Unmarshal(body, &res); err != nil || !res.OK { continue }
			switch res.Result.Status {
			case "creator":
				return "superadmin"
			case "administrator":
				result = "admin"
			}
			break // worked with this ID format, no need to try others
		}
	}
	return result
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
