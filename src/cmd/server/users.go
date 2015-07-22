package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
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

type userRegisterRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// UserRegister tries to create a new user inside the dash annotations database
func UserRegister(ctx context.Context, w http.ResponseWriter, req *http.Request) error {
	var db = ctx.Value(DBKey).(*sql.DB)

	var payload userRegisterRequest
	json.NewDecoder(req.Body).Decode(&payload)

	if _, err := findUserByUsername(db, payload.Username); err == nil {
		return ErrUsernameExists
	}

	var u = dash.User{
		Username: payload.Username,
	}
	u.ChangePassword(payload.Password)

	if _, err := db.Exec(`INSERT INTO users (username, password, created_at, updated_at) VALUES (?, ?, ?, ?)`, u.Username, u.EncryptedPassword, time.Now(), time.Now()); err != nil {
		return err
	}

	json.NewEncoder(w).Encode(map[string]string{
		"status": "success",
	})
	return nil
}

type userLoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func generateRandomBytes(n int) ([]byte, error) {
	var b = make([]byte, n)
	var _, err = rand.Read(b)
	if err != nil {
		return nil, err
	}
	return b, nil
}
func generateRandomString(s int) (string, error) {
	var b, err = generateRandomBytes(s)
	return base64.URLEncoding.EncodeToString(b), err
}

// UserLogin tries to authenticate an existing user using username/ password combination
func UserLogin(ctx context.Context, w http.ResponseWriter, req *http.Request) error {
	var db = ctx.Value(DBKey).(*sql.DB)

	var payload userLoginRequest
	json.NewDecoder(req.Body).Decode(&payload)

	var user, err = findUserByUsername(db, payload.Username)
	if err != nil {
		fmt.Printf("unknown user: %v", err)
		return ErrInvalidLogin
	}

	if !user.PasswordsMatch(payload.Password) {
		return ErrInvalidLogin
	}

	var sessionID, _ = generateRandomString(32)
	user.RememberToken = sql.NullString{
		String: sessionID,
		Valid:  true,
	}

	if _, err := db.Exec(`UPDATE users SET remember_token = ?, updated_at = ? WHERE id = ?`, user.RememberToken, time.Now(), user.ID); err != nil {
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
			Name: "laravel_session",
		}
	}

	if encryptedSessionID, err := encrypt([]byte(sessionID)); err != nil {
		return err
	} else {
		ckie.Value = string(encryptedSessionID)
	}
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

// UserLogout destroys the session of the current user
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

type userChangePasswordRequest struct {
	Password string `json:"password"`
}

// UserChangePassword changes the encrypted password of the current user
func UserChangePassword(ctx context.Context, w http.ResponseWriter, req *http.Request) error {
	var db = ctx.Value(DBKey).(*sql.DB)
	var user = ctx.Value(UserKey).(*dash.User)

	var payload userChangePasswordRequest
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

type userChangeEmailRequest struct {
	Email string `json:"email"`
}

// UserChangeEmail changes the email address of the current user
func UserChangeEmail(ctx context.Context, w http.ResponseWriter, req *http.Request) error {
	var db = ctx.Value(DBKey).(*sql.DB)
	var user = ctx.Value(UserKey).(*dash.User)

	var payload userChangeEmailRequest
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
