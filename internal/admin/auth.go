package admin

import (
	"encoding/json"
	"net/http"
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
