package app

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type userKey struct{}
type user struct{ Email, Role string }

// auth reproduces Python's OAuth2 bearer-token dependency semantics.
func (a *App) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h := r.Header.Get("Authorization")
		if !strings.HasPrefix(h, "Bearer ") {
			fail(w, 401, "Not authenticated")
			return
		}
		t, e := jwt.Parse(strings.TrimPrefix(h, "Bearer "), func(t *jwt.Token) (any, error) { return []byte(a.secret), nil })
		if e != nil || !t.Valid {
			fail(w, 401, "Could not validate credentials")
			return
		}
		sub, _ := t.Claims.(jwt.MapClaims)["sub"].(string)
		var role string
		if a.db.QueryRow(r.Context(), "SELECT role FROM users WHERE email=$1", sub).Scan(&role) != nil {
			fail(w, 401, "Could not validate credentials")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userKey{}, user{sub, role})))
	}
}
func (a *App) role(roles []string, next http.HandlerFunc) http.HandlerFunc {
	return a.auth(func(w http.ResponseWriter, r *http.Request) {
		u := r.Context().Value(userKey{}).(user)
		if u.Role != "Admin" {
			ok := false
			for _, v := range roles {
				if v == u.Role {
					ok = true
				}
			}
			if !ok {
				fail(w, 403, "Operation Denied: Insufficient authorization permissions.")
				return
			}
		}
		next(w, r)
	})
}
func (a *App) login(w http.ResponseWriter, r *http.Request) {
	if r.ParseForm() != nil {
		fail(w, 422, "Invalid form data")
		return
	}
	email, pw := r.Form.Get("username"), r.Form.Get("password")
	var hash, role string
	e := a.db.QueryRow(r.Context(), "SELECT password_hash,role FROM users WHERE email=$1", email).Scan(&hash, &role)
	if e != nil || ((strings.HasPrefix(hash, "$2a$") || strings.HasPrefix(hash, "$2b$")) && bcrypt.CompareHashAndPassword([]byte(hash), []byte(pw)) != nil) || (!strings.HasPrefix(hash, "$2") && hash != pw) {
		fail(w, 400, "Incorrect email or password")
		return
	}
	token, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sub": email, "exp": time.Now().UTC().Add(time.Hour).Unix()}).SignedString([]byte(a.secret))
	out(w, 200, map[string]any{"access_token": token, "token_type": "bearer", "role": role})
}
func (a *App) register(w http.ResponseWriter, r *http.Request) {
	var x struct{ Email, Password, Role string }
	if !decode(r, &x) {
		fail(w, 422, "Invalid request body")
		return
	}
	valid := map[string]bool{"Admin": true, "Fleet Manager": true, "Driver": true, "Safety Officer": true, "Financial Analyst": true}
	if !valid[x.Role] {
		fail(w, 400, "Invalid role. Must be one of: [Admin Fleet Manager Driver Safety Officer Financial Analyst]")
		return
	}
	var n int
	a.db.QueryRow(r.Context(), "SELECT count(*) FROM users WHERE email=$1", x.Email).Scan(&n)
	if n > 0 {
		fail(w, 400, "An account with this email is already registered.")
		return
	}
	h, _ := bcrypt.GenerateFromPassword([]byte(x.Password), bcrypt.DefaultCost)
	if _, e := a.db.Exec(r.Context(), "INSERT INTO users(email,password_hash,role) VALUES($1,$2,$3)", x.Email, string(h), x.Role); e != nil {
		fail(w, 500, "Internal Server Error")
		return
	}
	out(w, 201, map[string]any{"message": "User registered successfully", "email": x.Email, "role": x.Role})
}
