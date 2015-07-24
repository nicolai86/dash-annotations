package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"golang.org/x/net/context"

	"dash"
)

var (
	// ErrMissingUsername is returned for registration if missing the username parameter
	ErrMissingUsername = errors.New("Missing parameter: username")
	// ErrMissingPassword is returned for registration is missing the password parameter
	ErrMissingPassword = errors.New("Missing parameter: password")
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
	var payload userRegisterRequest
	json.NewDecoder(req.Body).Decode(&payload)

	if payload.Username == "" {
		return ErrMissingUsername
	}
	if payload.Password == "" {
		return ErrMissingPassword
	}

	var store = ctx.Value(UserStoreKey).(UserCreater)
	if err := store.InsertUser(payload.Username, payload.Password); err != nil {
		return err
	}

	w.WriteHeader(http.StatusCreated)
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
	var payload userLoginRequest
	json.NewDecoder(req.Body).Decode(&payload)

	if payload.Username == "" {
		return ErrMissingUsername
	}
	if payload.Password == "" {
		return ErrMissingPassword
	}

	var loginStore = ctx.Value(UserStoreKey).(UserLoginStore)
	var user, err = loginStore.FindUserByUsername(payload.Username)
	if err != nil {
		return ErrInvalidLogin
	}

	if !user.PasswordsMatch(payload.Password) {
		return ErrInvalidLogin
	}

	var sessionID, _ = generateRandomString(32)

	if err := loginStore.UpdateUserWithToken(user.Username, sessionID); err != nil {
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
	var tokenStore = ctx.Value(UserStoreKey).(UserTokenUpdater)
	var user = ctx.Value(UserKey).(*dash.User)

	http.SetCookie(w, &http.Cookie{
		Name:   "laravel_session",
		Value:  "",
		MaxAge: -1,
	})

	if err := tokenStore.UpdateUserWithToken(user.Username, ""); err != nil {
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
	var user = ctx.Value(UserKey).(*dash.User)
	var passwordUpdater = ctx.Value(UserStoreKey).(UserPasswordUpdater)

	var payload userChangePasswordRequest
	json.NewDecoder(req.Body).Decode(&payload)

	if err := passwordUpdater.UpdateUserWithPassword(user.Username, payload.Password); err != nil {
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
	var emailUpdater = ctx.Value(UserStoreKey).(UserEmailUpdater)
	var user = ctx.Value(UserKey).(*dash.User)

	var payload userChangeEmailRequest
	json.NewDecoder(req.Body).Decode(&payload)

	if err := emailUpdater.UpdateUserWithEmail(user.Username, payload.Email); err != nil {
		return err
	}

	json.NewEncoder(w).Encode(map[string]string{
		"status": "success",
	})
	return nil
}
