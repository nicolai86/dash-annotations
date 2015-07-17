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
	"strings"

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
const EntryKey key = 3

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

func FindEntryByID(db *sql.DB, entryID int) (dash.Entry, error) {
	var entry = dash.Entry{
		ID: entryID,
	}
	var rawTeams string
	var err = db.QueryRow(`SELECT
				e.title,
				e.type,
				e.anchor,
				e.body,
				e.body_rendered,
				e.score,
				e.user_id,
				e.removed_from_public,
				e.public,
				u.username,
				COALESCE((select group_concat(distinct t.name SEPARATOR '||||') FROM teams AS t inner join entry_team ON t.id = entry_team.team_id where entry_team.entry_id = e.id), '')
			FROM entries AS e
			INNER JOIN users AS u ON u.id = e.user_id
			WHERE e.id = ?`, entryID,
	).Scan(
		&entry.Title,
		&entry.Type,
		&entry.Anchor,
		&entry.Body,
		&entry.BodyRendered,
		&entry.Score,
		&entry.UserID,
		&entry.RemovedFromPublic,
		&entry.Public,
		&entry.AuthorUsername,
		&rawTeams)
	if rawTeams != "" {
		entry.Teams = strings.Split(rawTeams, "||||")
	}
	return entry, err
}

type withEntryPayload struct {
	EntryID int `json:"entry_id"`
}

// WithEntry is a middleware that extracts an entry from the given payload and
// adds it to the request context.
// The entry is searched by id, using the entry_id parameter
// If no entry was found the request is halted
func WithEntry(h ContextHandler) ContextHandler {
	return ContextHandlerFunc(func(ctx context.Context, rw http.ResponseWriter, req *http.Request) {
		var db = ctx.Value(DBKey).(*sql.DB)

		var enc = json.NewEncoder(rw)
		var body bytes.Buffer
		var dec = json.NewDecoder(io.TeeReader(req.Body, &body))
		req.Body = ioutil.NopCloser(&body)

		var payload withEntryPayload
		dec.Decode(&payload)

		if payload.EntryID == 0 {
			enc.Encode(map[string]string{
				"status":  "error",
				"message": "Missing entry_id parameter",
			})
			return
		}

		var entry, err = FindEntryByID(db, payload.EntryID)
		if err != nil {
			log.Printf("unable to find entry")
			enc.Encode(map[string]string{
				"status":  "error",
				"message": "Error. Logout and try again",
			})
			return
		}

		ctx = context.WithValue(ctx, EntryKey, &entry)
		h.ServeHTTPContext(ctx, rw, req)
	})
}

type withTeamPayload struct {
	TeamName string `json:"name"`
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

		var payload withTeamPayload
		dec.Decode(&payload)

		if payload.TeamName == "" {
			enc.Encode(map[string]string{
				"status":  "error",
				"message": "Missing name parameter",
			})
			return
		}

		var team, err = FindTeamByName(db, payload.TeamName)
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

func MaybeAuthenticated(h ContextHandler) ContextHandler {
	return ContextHandlerFunc(func(ctx context.Context, rw http.ResponseWriter, req *http.Request) {
		var db = ctx.Value(DBKey).(*sql.DB)

		var user = dash.User{}
		var sessionID = ""
		for _, cookie := range req.Cookies() {
			if cookie.Name == "laravel_session" {
				sessionID = cookie.Value
			}
		}
		if sessionID != "" {
			var err = db.QueryRow(`SELECT id, username, email, password, moderator FROM users WHERE remember_token = ?`, sessionID).Scan(&user.ID, &user.Username, &user.Email, &user.EncryptedPassword, &user.Moderator)
			if err == nil {
				ctx = context.WithValue(ctx, UserKey, &user)
			}
		}

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
