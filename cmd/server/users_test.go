package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nicolai86/dash-annotations/dash"
)

func TestUserRegister_HappyPath(t *testing.T) {
	var mock = mockUserLoginStore{
		insertUser: func(username, password string) error {
			if username != "example" {
				t.Errorf("Expected to update user %q but was %q", "example", username)
			}
			if password != "supersecret" {
				t.Errorf("Expected to update password to %q but was %q", "supersecret", password)
			}
			return nil
		},
	}

	var ctx = context.WithValue(rootCtx, UserStoreKey, &mock)

	req, _ := http.NewRequest("POST", "/users/register", strings.NewReader(`{"username": "example", "password": "supersecret"}`))
	rw := httptest.NewRecorder()

	if err := UserRegister(ctx, rw, req); err != nil {
		t.Fatalf("Registration failed with %v", err)
	}

	if rw.Code != http.StatusCreated {
		t.Errorf("Non-expected status code%v:\n\tbody: %+v", http.StatusCreated, rw.Code)
	}

	var data map[string]string
	json.NewDecoder(rw.Body).Decode(&data)
	if data["status"] != "success" {
		t.Errorf("Expected status of %q to be %q", data["status"], "success")
	}
}

func TestUserRegister_MissingUsername(t *testing.T) {
	t.Parallel()

	var req, _ = http.NewRequest("POST", "/users/register", strings.NewReader(`{"password": "example"}`))
	var rw = httptest.NewRecorder()

	if err := UserRegister(rootCtx, rw, req); err != ErrMissingUsername {
		t.Fatalf("Username is missing, but errored with: %v", err)
	}
}

func TestUserRegister_MissingPassword(t *testing.T) {
	t.Parallel()

	req, _ := http.NewRequest("POST", "/users/register", strings.NewReader(`{"username": "example"}`))
	rw := httptest.NewRecorder()

	if err := UserRegister(rootCtx, rw, req); err != ErrMissingPassword {
		t.Fatalf("Password is missing, but errored with: %v", err)
	}
}

type mockUserLoginStore struct {
	findUserByUsername     func() (dash.User, error)
	updateUserWithToken    func(username, token string) error
	updateUserWithPassword func(username, password string) error
	updateUserWithEmail    func(username, email string) error
	insertUser             func(username, password string) error
}

func (mock *mockUserLoginStore) FindUserByUsername(username string) (dash.User, error) {
	return mock.findUserByUsername()
}

func (mock *mockUserLoginStore) UpdateUserWithToken(username, token string) error {
	return mock.updateUserWithToken(username, token)
}

func (mock *mockUserLoginStore) UpdateUserWithPassword(username, password string) error {
	return mock.updateUserWithPassword(username, password)
}

func (mock *mockUserLoginStore) UpdateUserWithEmail(username, email string) error {
	return mock.updateUserWithEmail(username, email)
}

func (mock *mockUserLoginStore) InsertUser(username, password string) error {
	return mock.insertUser(username, password)
}

func TestUserLogin_HappyPath(t *testing.T) {
	var user = dash.User{
		Username:          "max mustermann",
		Email:             sql.NullString{String: "max@mustermann.de"},
		EncryptedPassword: encryptPassword("musterpasswort"),
	}

	var mock = mockUserLoginStore{
		findUserByUsername: func() (dash.User, error) {
			return user, nil
		},
		updateUserWithToken: func(_, _ string) error {
			return nil
		},
	}

	var ctx = context.WithValue(rootCtx, UserStoreKey, &mock)

	req, _ := http.NewRequest("POST", "/users/login", strings.NewReader(`{"username": "max mustermann", "password": "musterpasswort"}`))
	rw := httptest.NewRecorder()

	if err := UserLogin(ctx, rw, req); err != nil {
		t.Fatalf("UserLogin failed with: %#v", err)
	}
	if rw.Code != http.StatusOK {
		t.Errorf("Non-expected status code%v:\n\tbody: %+v", http.StatusOK, rw.Code)
	}

	var data map[string]string
	json.NewDecoder(rw.Body).Decode(&data)
	if data["email"] != user.Email.String {
		t.Errorf("Expected email of %q to be %q", data["email"], user.Email.String)
	}
	if data["status"] != "success" {
		t.Errorf("Expected status of %q to be %q", data["status"], "success")
	}

	if !strings.Contains(rw.Header().Get("Set-Cookie"), "laravel_session") {
		t.Errorf("Expected Set-Cookie header to contain %v", "laravel_session")
	}
}

func TestUserLogin_UnknownUser(t *testing.T) {
	var mock = mockUserLoginStore{
		findUserByUsername: func() (dash.User, error) {
			return dash.User{}, ErrUnknownUser
		},
	}
	var ctx = context.WithValue(rootCtx, UserStoreKey, &mock)

	req, _ := http.NewRequest("POST", "/users/login", strings.NewReader(`{"username": "mistyped", "password": "musterpasswort"}`))
	rw := httptest.NewRecorder()

	if err := UserLogin(ctx, rw, req); err != ErrInvalidLogin {
		t.Fatalf("UserLogin with unknown user errored with: %#v", err)
	}
}

func TestUserLogin_PasswordMismatch(t *testing.T) {
	var mock = mockUserLoginStore{
		findUserByUsername: func() (dash.User, error) {
			return dash.User{Username: "correct"}, nil
		},
	}
	var ctx = context.WithValue(rootCtx, UserStoreKey, &mock)

	req, _ := http.NewRequest("POST", "/users/login", strings.NewReader(`{"username": "correct", "password": "wrong"}`))
	rw := httptest.NewRecorder()

	if err := UserLogin(ctx, rw, req); err != ErrInvalidLogin {
		t.Fatalf("UserLogin with unknown user errored with: %#v", err)
	}
}

func TestUserLogout_HappyPath(t *testing.T) {
	var currentUser = dash.User{Username: "tester"}
	var ctx = context.WithValue(rootCtx, UserKey, &currentUser)

	var mock = mockUserLoginStore{
		updateUserWithToken: func(username, token string) error {
			if username != currentUser.Username {
				t.Errorf("Expected to update user %q but was %q", currentUser.Username, username)
			}
			return nil
		},
	}
	ctx = context.WithValue(ctx, UserStoreKey, &mock)

	req, _ := http.NewRequest("POST", "/users/logout", strings.NewReader(``))
	rw := httptest.NewRecorder()

	if err := UserLogout(ctx, rw, req); err != nil {
		t.Fatalf("UserLogout errored with: %#v", err)
	}

	if !strings.Contains(rw.Header().Get("Set-Cookie"), "laravel_session=; Max-Age=0") {
		t.Errorf("Expected Set-Cookie header to contain %v", "laravel_session")
	}

	var data map[string]string
	json.NewDecoder(rw.Body).Decode(&data)
	if data["status"] != "success" {
		t.Errorf("Expected status of %q to be %q", data["status"], "success")
	}
}

func TestUserChangePassword_HappyPath(t *testing.T) {
	var currentUser = dash.User{Username: "tester"}
	var ctx = context.WithValue(rootCtx, UserKey, &currentUser)

	var mock = mockUserLoginStore{
		updateUserWithPassword: func(username, password string) error {
			if username != currentUser.Username {
				t.Errorf("Expected to update user %q but was %q", currentUser.Username, username)
			}
			if password != "supersecret" {
				t.Errorf("Expected to update password to %q but was %q", "supersecret", password)
			}
			return nil
		},
	}
	ctx = context.WithValue(ctx, UserStoreKey, &mock)

	req, _ := http.NewRequest("POST", "/users/logout", strings.NewReader(`{"password":"supersecret"}`))
	rw := httptest.NewRecorder()

	if err := UserChangePassword(ctx, rw, req); err != nil {
		t.Fatalf("UserChangePassword errored with: %#v", err)
	}
	var data map[string]string
	json.NewDecoder(rw.Body).Decode(&data)
	if data["status"] != "success" {
		t.Errorf("Expected status of %q to be %q", data["status"], "success")
	}
}

func TestUserChangeEmail_HappyPath(t *testing.T) {
	var currentUser = dash.User{Username: "tester"}
	var ctx = context.WithValue(rootCtx, UserKey, &currentUser)

	var mock = mockUserLoginStore{
		updateUserWithEmail: func(username, email string) error {
			if username != currentUser.Username {
				t.Errorf("Expected to update user %q but was %q", currentUser.Username, username)
			}
			if email != "max@mustermann.de" {
				t.Errorf("Expected to update password to %q but was %q", "max@mustermann.de", email)
			}
			return nil
		},
	}
	ctx = context.WithValue(ctx, UserStoreKey, &mock)

	req, _ := http.NewRequest("POST", "/users/change_email", strings.NewReader(`{"email":"max@mustermann.de"}`))
	rw := httptest.NewRecorder()

	if err := UserChangeEmail(ctx, rw, req); err != nil {
		t.Fatalf("UserChangeEmail errored with: %#v", err)
	}
	var data map[string]string
	json.NewDecoder(rw.Body).Decode(&data)
	if data["status"] != "success" {
		t.Errorf("Expected status of %q to be %q", data["status"], "success")
	}
}
