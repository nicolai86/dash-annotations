package handlers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
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
const TeamKey key = 2

type ContextHandler interface {
	ServeHTTPContext(context.Context, http.ResponseWriter, *http.Request)
}

type ContextHandlerFunc func(context.Context, http.ResponseWriter, *http.Request)

func (h ContextHandlerFunc) ServeHTTPContext(ctx context.Context, rw http.ResponseWriter, req *http.Request) {
	h(ctx, rw, req)
}

// NewRootContext returns a context with the database set. This serves as the root
// context for all other contexts
func NewRootContext(db *sql.DB) context.Context {
	return context.WithValue(context.Background(), DBKey, db)
}

func FindTeamByName(db *sql.DB, teamName string) (dash.Team, error) {
	var team = dash.Team{
		Name: teamName,
	}
	var err = db.QueryRow(`SELECT t.id, t.access_key, tm.user_id FROM teams AS t INNER JOIN team_user AS tm ON tm.team_id = t.id WHERE name = ? AND tm.role = ? LIMIT 1`, teamName, "owner").Scan(&team.ID, &team.EncryptedAccessKey, &team.OwnerID)
	return team, err
}

func FindUserByUsername(db *sql.DB, username string) (dash.User, error) {
	var user = dash.User{
		Username: username,
	}
	var err = db.QueryRow(`SELECT id, email, password, remember_token, moderator FROM users WHERE username = ?`, username).Scan(&user.ID, &user.Email, &user.EncryptedPassword, &user.RememberToken, &user.Moderator)
	return user, err
}

// WithTeam is a middleware that extracts the team from the given payload and
// adds it to the request context.
// The team is always searched by using the name parameter
// If no team is found the request is halted.
func WithTeam(h ContextHandler) ContextHandler {
	return ContextHandlerFunc(func(ctx context.Context, rw http.ResponseWriter, req *http.Request) {
		var db = ctx.Value(DBKey).(*sql.DB)

		var enc = json.NewEncoder(rw)
		var body bytes.Buffer
		var dec = json.NewDecoder(io.TeeReader(req.Body, &body))
		req.Body = ioutil.NopCloser(&body)

		var payload map[string]interface{}
		dec.Decode(&payload)

		var teamName, ok = payload["name"]
		if !ok {
			enc.Encode(map[string]string{
				"status":  "error",
				"message": "Missing name parameter",
			})
			return
		}

		var team, err = FindTeamByName(db, teamName.(string))
		if err != nil {
			enc.Encode(map[string]string{
				"status":  "error",
				"message": "Team does not exist",
			})
			return
		}

		ctx = context.WithValue(ctx, TeamKey, &team)
		h.ServeHTTPContext(ctx, rw, req)
	})
}

// Authenticated is a middleware that checks for authentication in the request
// Authentication is identified using the laravel_session cookie.
// If no authentication is present the request is halted.
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
