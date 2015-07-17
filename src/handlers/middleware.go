package handlers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	"dash"

	"golang.org/x/net/context"
)

// NewRootContext returns a context with the database set. This serves as the root
// context for all other contexts
func NewRootContext(db *sql.DB) context.Context {
	return context.WithValue(context.Background(), DBKey, db)
}

// ContextHandler allows http Handlers to includea context
type ContextHandler interface {
	ServeHTTPContext(context.Context, http.ResponseWriter, *http.Request) error
}

// ContextHandlerFunc defines a function that implements the ContextHandler interface
type ContextHandlerFunc func(context.Context, http.ResponseWriter, *http.Request) error

// ServeHTTPContext calls the ContextHandlerFunc with the given context, ResponseWrite and Request
func (h ContextHandlerFunc) ServeHTTPContext(ctx context.Context, rw http.ResponseWriter, req *http.Request) error {
	return h(ctx, rw, req)
}

// The key type is unexported to prevent collisions with context keys defined in
// other packages.
type key int

// UserKey is used to fetch the current user from a context
const UserKey key = 0

// DBKey is used to fetch the database connection from a context
const DBKey key = 1

// TeamKey is used to fetch the current team from a context
const TeamKey key = 2

// EntryKey is used to fetch the current entry from a context
const EntryKey key = 3

type withEntryPayload struct {
	EntryID int `json:"entry_id"`
}

func findEntryByID(db *sql.DB, entryID int) (dash.Entry, error) {
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

// WithEntry is a middleware that extracts an entry from the given payload and
// adds it to the request context.
// The entry is searched by id, using the entry_id parameter
// If no entry was found the request is halted
func WithEntry(h ContextHandler) ContextHandler {
	return ContextHandlerFunc(func(ctx context.Context, rw http.ResponseWriter, req *http.Request) error {
		var db = ctx.Value(DBKey).(*sql.DB)

		var body bytes.Buffer
		var dec = json.NewDecoder(io.TeeReader(req.Body, &body))
		req.Body = ioutil.NopCloser(&body)

		var payload withEntryPayload
		dec.Decode(&payload)

		if payload.EntryID == 0 {
			return errors.New("Missing parameter: entry_id")
		}

		var entry, err = findEntryByID(db, payload.EntryID)
		if err != nil {
			return errors.New("Unknown entry")
		}

		ctx = context.WithValue(ctx, EntryKey, &entry)
		return h.ServeHTTPContext(ctx, rw, req)
	})
}

type withTeamPayload struct {
	TeamName string `json:"name"`
}

func findTeamByName(db *sql.DB, teamName string) (dash.Team, error) {
	var team = dash.Team{
		Name: teamName,
	}
	var err = db.QueryRow(`SELECT t.id, t.access_key, tm.user_id FROM teams AS t INNER JOIN team_user AS tm ON tm.team_id = t.id WHERE name = ? AND tm.role = ? LIMIT 1`, teamName, "owner").Scan(&team.ID, &team.EncryptedAccessKey, &team.OwnerID)
	return team, err
}

// WithTeam is a middleware that extracts the team from the given payload and
// adds it to the request context.
// The team is always searched by using the name parameter
// If no team is found the request is halted.
func WithTeam(h ContextHandler) ContextHandler {
	return ContextHandlerFunc(func(ctx context.Context, rw http.ResponseWriter, req *http.Request) error {
		var db = ctx.Value(DBKey).(*sql.DB)

		var body bytes.Buffer
		var dec = json.NewDecoder(io.TeeReader(req.Body, &body))
		req.Body = ioutil.NopCloser(&body)

		var payload withTeamPayload
		dec.Decode(&payload)

		if payload.TeamName == "" {
			return ErrTeamNameMissing
		}

		var team, err = findTeamByName(db, payload.TeamName)
		if err != nil {
			return errors.New("Unknown team name")
		}

		ctx = context.WithValue(ctx, TeamKey, &team)
		return h.ServeHTTPContext(ctx, rw, req)
	})
}

func findUserByUsername(db *sql.DB, username string) (dash.User, error) {
	var user = dash.User{
		Username: username,
	}
	var err = db.QueryRow(`SELECT id, email, password, remember_token, moderator FROM users WHERE username = ?`, username).Scan(&user.ID, &user.Email, &user.EncryptedPassword, &user.RememberToken, &user.Moderator)
	return user, err
}

// Authenticated is a middleware that checks for authentication in the request
// Authentication is identified using the laravel_session cookie.
// If no authentication is present the request is halted.
func Authenticated(h ContextHandler) ContextHandler {
	return ContextHandlerFunc(func(ctx context.Context, rw http.ResponseWriter, req *http.Request) error {
		var db = ctx.Value(DBKey).(*sql.DB)

		var user = dash.User{}
		var sessionID = ""
		for _, cookie := range req.Cookies() {
			if cookie.Name == "laravel_session" {
				sessionID = cookie.Value
			}
		}
		if sessionID == "" {
			return errors.New("Missing session cookie")
		}

		if err := db.QueryRow(`SELECT id, username, email, password, moderator FROM users WHERE remember_token = ?`, sessionID).Scan(&user.ID, &user.Username, &user.Email, &user.EncryptedPassword, &user.Moderator); err != nil {
			return err
		}
		ctx = context.WithValue(ctx, UserKey, &user)

		return h.ServeHTTPContext(ctx, rw, req)
	})
}

func MaybeAuthenticated(h ContextHandler) ContextHandler {
	return ContextHandlerFunc(func(ctx context.Context, rw http.ResponseWriter, req *http.Request) error {
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

		return h.ServeHTTPContext(ctx, rw, req)
	})
}
