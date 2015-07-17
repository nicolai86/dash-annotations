package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"math/rand"
	"net/http"
	"time"

	"golang.org/x/net/context"

	"dash"
)

var (
	// ErrUsernameExists is returned for registration attempts where the username is taken
	ErrUsernameExists = errors.New("A user with this username already exists")
	// ErrInvalidLogin is returned if the login request fails, either because the username or password is wrong
	ErrInvalidLogin = errors.New("Login failed: invalid username or password")
	// ErrEmailExists is returned when a user wants to change his email to an already taken email address
	ErrEmailExists = errors.New("A user with this email already exists")
)

type registerRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func UsersRegister(ctx context.Context, w http.ResponseWriter, req *http.Request) error {
	var db = ctx.Value(DBKey).(*sql.DB)

	var payload registerRequest
	json.NewDecoder(req.Body).Decode(&payload)

	if _, err := findUserByUsername(db, payload.Username); err == nil {
		return ErrUsernameExists
	}

	var u = dash.User{
		Username: payload.Username,
	}
	u.ChangePassword(payload.Password)

	if _, err := db.Exec(`INSERT INTO users (username, password, created_at) VALUES (?, ?, ?)`, u.Username, u.EncryptedPassword, time.Now()); err != nil {
		return err
	}

	json.NewEncoder(w).Encode(map[string]string{
		"status": "success",
	})
	return nil
}

var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

func randSeq(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func UserLogin(ctx context.Context, w http.ResponseWriter, req *http.Request) error {
	var db = ctx.Value(DBKey).(*sql.DB)

	var payload loginRequest
	json.NewDecoder(req.Body).Decode(&payload)

	var user, err = findUserByUsername(db, payload.Username)
	if err != nil {
		return ErrInvalidLogin
	}

	if !user.PasswordsMatch(payload.Password) {
		return ErrInvalidLogin
	}

	user.RememberToken = sql.NullString{
		String: randSeq(32),
		Valid:  true,
	}

	if _, err := db.Exec(`UPDATE users SET remember_token = ? WHERE id = ?`, user.RememberToken, user.ID); err != nil {
		return err
	}

	var ckie *http.Cookie
	for _, cookie := range req.Cookies() {
		if cookie.Name == "laravel_session" {
			ckie = cookie
			break
		}
	}

	if ckie == nil {
		ckie = &http.Cookie{
			Name:  "laravel_session",
			Value: user.RememberToken.String,
		}
	}

	ckie.Value = user.RememberToken.String
	ckie.MaxAge = 7200
	ckie.Expires = time.Now().Add(7200 * time.Second)
	ckie.Path = "/"
	ckie.HttpOnly = true
	http.SetCookie(w, ckie)

	w.WriteHeader(http.StatusOK)
	var data = map[string]string{
		"status": "success",
	}
	if user.Email.String != "" {
		data["email"] = user.Email.String
	}
	json.NewEncoder(w).Encode(data)
	return nil
}

func UserLogout(ctx context.Context, w http.ResponseWriter, req *http.Request) error {
	var db = ctx.Value(DBKey).(*sql.DB)
	var user = ctx.Value(UserKey).(*dash.User)

	http.SetCookie(w, &http.Cookie{
		Name:   "laravel_session",
		Value:  "",
		MaxAge: -1,
	})

	user.RememberToken = sql.NullString{
		Valid: true,
	}

	if _, err := db.Exec(`UPDATE users SET remember_token = ?, updated_at = ? WHERE id = ?`, user.RememberToken, time.Now(), user.ID); err != nil {
		return err
	}

	json.NewEncoder(w).Encode(map[string]string{
		"status": "success",
	})
	return nil
}

type changePasswordRequest struct {
	Password string `json:"password"`
}

func UserChangePassword(ctx context.Context, w http.ResponseWriter, req *http.Request) error {
	var db = ctx.Value(DBKey).(*sql.DB)
	var user = ctx.Value(UserKey).(*dash.User)

	var payload changePasswordRequest
	json.NewDecoder(req.Body).Decode(&payload)

	user.ChangePassword(payload.Password)
	if _, err := db.Exec(`UPDATE users SET password = ?, updated_at = ? WHERE id = ?`, user.EncryptedPassword, time.Now(), user.ID); err != nil {
		return err
	}

	json.NewEncoder(w).Encode(map[string]string{
		"status": "success",
	})
	return nil
}

type changeEmailRequest struct {
	Email string `json:"email"`
}

func UserChangeEmail(ctx context.Context, w http.ResponseWriter, req *http.Request) error {
	var db = ctx.Value(DBKey).(*sql.DB)
	var user = ctx.Value(UserKey).(*dash.User)

	var payload changeEmailRequest
	json.NewDecoder(req.Body).Decode(&payload)

	var existingUserID = -1
	db.QueryRow(`SELECT id FROM users WHERE email = ?`, payload.Email).Scan(&existingUserID)
	if existingUserID != -1 {
		return ErrEmailExists
	}

	user.Email = sql.NullString{String: payload.Email, Valid: true}
	if _, err := db.Exec(`UPDATE users SET email = ?, updated_at = ? WHERE id = ?`, user.Email, time.Now(), user.ID); err != nil {
		return err
	}

	json.NewEncoder(w).Encode(map[string]string{
		"status": "success",
	})
	return nil
}
