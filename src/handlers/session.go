package handlers

import (
	"database/sql"
	"errors"
	"log"
	"net/http"

	userStore "user_storage"

	"dash"

	"golang.org/x/net/context"
)

var (
	// ErrNotLoggedIn is returned when authentication is required but not present
	ErrNotLoggedIn = errors.New("You are not logged in")
)

// The key type is unexported to prevent collisions with context keys defined in
// other packages.
type key int

const UserKey key = 0
const DBKey key = 1

type ContextHandler interface {
	ServeHTTPContext(context.Context, http.ResponseWriter, *http.Request)
}

type ContextHandlerFunc func(context.Context, http.ResponseWriter, *http.Request)

func (h ContextHandlerFunc) ServeHTTPContext(ctx context.Context, rw http.ResponseWriter, req *http.Request) {
	h(ctx, rw, req)
}

func NewRootContext(db *sql.DB) context.Context {
	return context.WithValue(context.Background(), DBKey, db)
}

func Authenticated(h ContextHandler) ContextHandler {
	return ContextHandlerFunc(func(ctx context.Context, rw http.ResponseWriter, req *http.Request) {
		var db = ctx.Value(DBKey).(*sql.DB)

		var user = dash.User{}
		var sessionID = ""
		for _, cookie := range req.Cookies() {
			if cookie.Name == "laravel_session" {
				sessionID = cookie.Value
			}
		}
		if sessionID == "" {
			log.Printf("no laravel_session cookie found")
			return
		}
		var err = db.QueryRow(`SELECT id, username, email, password, moderator FROM users WHERE remember_token = ?`, sessionID).Scan(&user.ID, &user.Username, &user.Email, &user.EncryptedPassword, &user.Moderator)
		if err != nil {
			log.Printf("unknown user")
			return
		}
		ctx = context.WithValue(ctx, UserKey, &user)

		h.ServeHTTPContext(ctx, rw, req)
	})
}

func getUserFromSession(u userStore.Storage, req *http.Request) (dash.User, error) {
	var sessionID = ""
	for _, cookie := range req.Cookies() {
		if cookie.Name == "laravel_session" {
			sessionID = cookie.Value
		}
	}
	if sessionID == "" {
		log.Printf("no laravel_session cookie found")
		return dash.User{}, errors.New("no session")
	}
	log.Printf("session: %q", sessionID)
	return u.FindByRememberToken(sessionID)
}
