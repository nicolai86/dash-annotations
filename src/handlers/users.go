package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"time"

	"golang.org/x/net/context"

	"dash"
)

func UsersRegister(ctx context.Context, w http.ResponseWriter, req *http.Request) {
	var db = ctx.Value(DBKey).(*sql.DB)

	var dec = json.NewDecoder(req.Body)
	var payload loginRequest
	dec.Decode(&payload)

	var enc = json.NewEncoder(w)

	if _, err := FindUserByUsername(db, payload.Username); err == nil {
		log.Printf("bad register request: %#v", payload)
		w.WriteHeader(http.StatusBadRequest)
		enc.Encode(map[string]string{
			"status":  "error",
			"message": "Username already taken",
		})
		return
	}

	var u = dash.User{
		Username: payload.Username,
	}
	u.ChangePassword(payload.Password)

	var res, err = db.Exec(`INSERT INTO users (username, password, created_at) VALUES (?, ?, ?)`, u.Username, u.EncryptedPassword, time.Now())
	if err != nil {
		log.Printf("err: %v", err)
		return
	}
	var userID int64
	userID, err = res.LastInsertId()
	if err != nil {
		log.Printf("err: %v", err)
		return
	}
	u.ID = int(userID)

	log.Printf("registration successful")
	w.WriteHeader(http.StatusOK)
	enc.Encode(map[string]string{
		"status": "success",
	})
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

func randSeq(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func UserLogin(ctx context.Context, w http.ResponseWriter, req *http.Request) {
	var db = ctx.Value(DBKey).(*sql.DB)

	var dec = json.NewDecoder(req.Body)
	var payload loginRequest
	dec.Decode(&payload)

	var enc = json.NewEncoder(w)
	var u, err = FindUserByUsername(db, payload.Username)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		enc.Encode(map[string]string{
			"status":  "error",
			"message": "Invalid username or password",
		})
		return
	}

	if !u.PasswordsMatch(payload.Password) {
		w.WriteHeader(http.StatusBadRequest)
		enc.Encode(map[string]string{
			"status":  "error",
			"message": "Invalid username or password",
		})
		return
	}

	u.RememberToken = sql.NullString{
		String: randSeq(32),
		Valid:  true,
	}

	if _, err := db.Exec(`UPDATE users SET remember_token = ? WHERE id = ?`, u.RememberToken, u.ID); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		log.Printf("unable to create session")
		enc.Encode(map[string]string{
			"status": "error",
		})
		return
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
			Value: u.RememberToken.String,
		}
	}

	ckie.Value = u.RememberToken.String
	ckie.MaxAge = 7200
	ckie.Expires = time.Now().Add(7200 * time.Second)
	ckie.Path = "/"
	ckie.HttpOnly = true
	http.SetCookie(w, ckie)

	log.Printf("login successful: %q", u.RememberToken.String)
	w.WriteHeader(http.StatusOK)
	var data = map[string]string{
		"status": "success",
	}
	if u.Email.String != "" {
		data["email"] = u.Email.String
	}
	enc.Encode(data)
}

func UserLogout(ctx context.Context, w http.ResponseWriter, req *http.Request) {
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

	db.Exec(`UPDATE users SET remember_token = ?, updated_at = ? WHERE id = ?`, user.RememberToken, time.Now(), user.ID)
	log.Printf("logout successful")

	var enc = json.NewEncoder(w)
	enc.Encode(map[string]string{
		"status": "success",
	})
}

type changePasswordRequest struct {
	Password string `json:"password"`
}

func UserChangePassword(ctx context.Context, w http.ResponseWriter, req *http.Request) {
	var db = ctx.Value(DBKey).(*sql.DB)
	var user = ctx.Value(UserKey).(*dash.User)

	var payload changePasswordRequest
	var dec = json.NewDecoder(req.Body)
	dec.Decode(&payload)

	user.ChangePassword(payload.Password)
	db.Exec(`UPDATE users SET password = ?, updated_at = ? WHERE id = ?`, user.EncryptedPassword, time.Now(), user.ID)

	var enc = json.NewEncoder(w)
	enc.Encode(map[string]string{
		"status": "success",
	})
}

type changeEmailRequest struct {
	Email string `json:"email"`
}

func UserChangeEmail(ctx context.Context, w http.ResponseWriter, req *http.Request) {
	var db = ctx.Value(DBKey).(*sql.DB)
	var user = ctx.Value(UserKey).(*dash.User)

	var enc = json.NewEncoder(w)

	var payload changeEmailRequest
	var dec = json.NewDecoder(req.Body)
	dec.Decode(&payload)

	var existingUserID = -1
	db.QueryRow(`SELECT id FROM users WHERE email = ?`, payload.Email).Scan(&existingUserID)
	if existingUserID != -1 {
		w.WriteHeader(http.StatusUnauthorized)
		enc.Encode(map[string]string{
			"status":  "error",
			"message": "Email already used",
		})
		return
	}

	user.Email = sql.NullString{String: payload.Email, Valid: true}
	db.Exec(`UPDATE users SET email = ?, updated_at = ? WHERE id = ?`, user.Email, time.Now(), user.ID)

	enc.Encode(map[string]string{
		"status": "success",
	})
}
