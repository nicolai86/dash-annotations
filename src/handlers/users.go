package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"time"

	userStore "user_storage"

	"dash"
)

type UsersHandler struct {
	UserStorage userStore.Storage
}

func (ux *UsersHandler) isValidRegistrationAttempt(l loginRequest) bool {
	var _, err = ux.UserStorage.FindByUsername(l.Username)
	return err != nil
}

func (ux *UsersHandler) register(w http.ResponseWriter, req *http.Request) {
	var dec = json.NewDecoder(req.Body)
	var payload loginRequest
	dec.Decode(&payload)

	var enc = json.NewEncoder(w)
	if !ux.isValidRegistrationAttempt(payload) {
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
	ux.UserStorage.Store(&u)

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

func (ux *UsersHandler) login(w http.ResponseWriter, req *http.Request) {
	var dec = json.NewDecoder(req.Body)
	var payload loginRequest
	dec.Decode(&payload)

	var enc = json.NewEncoder(w)
	if _, err := ux.UserStorage.FindByUsername(payload.Username); err != nil {
		log.Printf("bad login request: %#v, %#v", payload, err)
		w.WriteHeader(http.StatusBadRequest)
		enc.Encode(map[string]string{
			"status":  "error",
			"message": "Invalid username or password",
		})
		return
	}

	var u, _ = ux.UserStorage.FindByUsername(payload.Username)
	log.Printf("%#v", u)
	if !u.PasswordsMatch(payload.Password) {
		log.Printf("bad login request: %#v", payload)
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

	if e := ux.UserStorage.Update(&u); e != nil {
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

func (ux *UsersHandler) logout(w http.ResponseWriter, req *http.Request) {
	var sessionID = ""
	for _, cookie := range req.Cookies() {
		if cookie.Name == "laravel_session" {
			sessionID = cookie.Value
		}
	}

	http.SetCookie(w, &http.Cookie{
		Name:   "laravel_session",
		Value:  "",
		MaxAge: -1,
	})

	var enc = json.NewEncoder(w)
	if sessionID == "" {
		enc.Encode(map[string]string{
			"status":  "error",
			"message": "Not logged in",
		})
		return
	}

	var u, _ = ux.UserStorage.FindByRememberToken(sessionID)
	u.RememberToken = sql.NullString{
		Valid: true,
	}
	ux.UserStorage.Update(&u)
	log.Printf("logout successful")

	enc.Encode(map[string]string{
		"status": "success",
	})
}

type changePasswordRequest struct {
	Password string `json:"password"`
}

func (ux *UsersHandler) changePassword(w http.ResponseWriter, req *http.Request) {
	var u, err = getUserFromSession(ux.UserStorage, req)
	var enc = json.NewEncoder(w)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		enc.Encode(map[string]string{
			"status":  "error",
			"message": "Error. Logout and try again",
		})
		return
	}

	var payload changePasswordRequest
	var dec = json.NewDecoder(req.Body)
	dec.Decode(&payload)

	u.ChangePassword(payload.Password)
	ux.UserStorage.Update(&u)

	enc.Encode(map[string]string{
		"status": "success",
	})
}

type changeEmailRequest struct {
	Email string `json:"email"`
}

func (ux *UsersHandler) changeEmail(w http.ResponseWriter, req *http.Request) {
	var u, err = getUserFromSession(ux.UserStorage, req)
	var enc = json.NewEncoder(w)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		enc.Encode(map[string]string{
			"status":  "error",
			"message": "Error. Logout and try again",
		})
		return
	}

	var payload changeEmailRequest
	var dec = json.NewDecoder(req.Body)
	dec.Decode(&payload)

	var _, err2 = ux.UserStorage.FindByEmail(payload.Email)
	if err2 == nil {
		w.WriteHeader(http.StatusUnauthorized)
		enc.Encode(map[string]string{
			"status":  "error",
			"message": "Email already used",
		})
		return
	}

	u.Email = sql.NullString{String: payload.Email, Valid: true}
	ux.UserStorage.Update(&u)

	enc.Encode(map[string]string{
		"status": "success",
	})
}

func (ux *UsersHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	switch req.URL.Path {
	case "register":
		ux.register(w, req)
	case "login":
		ux.login(w, req)
	case "logout":
		ux.logout(w, req)
	case "password":
		ux.changePassword(w, req)
	case "email":
		ux.changeEmail(w, req)
	case "forgot/request":
		log.Printf("forgot/request not implemented")
	case "forgot/reset":
		log.Printf("forgot/reset not implemented")
	default:
		log.Printf("Unknown users route: %v", req.URL.Path)
	}
}
