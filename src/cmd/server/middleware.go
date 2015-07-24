package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"net/http"

	"dash"

	"golang.org/x/net/context"
)

var (
	encryptionKey = "1234567812345678"
	// ErrAuthenticationRequired is returned from Authenticated middleware if the session can not be matched to an existing user
	ErrAuthenticationRequired = errors.New("Authentication required")
	// ErrTeamUnknown is returned when a name cannot be matched to the requested team name
	ErrTeamUnknown = errors.New("Unknown team")
	// ErrEntryUnknown is returned if the entry_id cannot be matched to the requested entry_id
	ErrEntryUnknown = errors.New("Unknown entry")
	// ErrMissingEntryID is returned if the entry_id parameter is empty or not present
	ErrMissingEntryID = errors.New("Missing parameter: entry_id")
)

func encrypt(b []byte) ([]byte, error) {
	var block, err = aes.NewCipher([]byte(encryptionKey))

	if err != nil {
		return nil, err
	}

	encrypted := make([]byte, aes.BlockSize+len(b))
	iv := encrypted[:aes.BlockSize]

	encrypter := cipher.NewCFBEncrypter(block, iv)
	encrypter.XORKeyStream(encrypted[aes.BlockSize:], b)

	var encoded = make([]byte, base64.URLEncoding.EncodedLen(len(encrypted)))
	base64.URLEncoding.Encode(encoded, encrypted)

	return encoded, nil
}

func decrypt(encrypted []byte) ([]byte, error) {
	var decoded = make([]byte, base64.URLEncoding.DecodedLen(len(encrypted)))
	var n, _ = base64.URLEncoding.Decode(decoded, encrypted)
	encrypted = decoded[:n]

	var block, err = aes.NewCipher([]byte(encryptionKey))

	if err != nil {
		return nil, err
	}

	iv := encrypted[:aes.BlockSize]

	encrypted = encrypted[aes.BlockSize:]

	decrypter := cipher.NewCFBDecrypter(block, iv)

	decrypted := make([]byte, len(encrypted))
	decrypter.XORKeyStream(decrypted, encrypted)

	return decrypted, nil
}

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

// UserStoreKey is used to fetch a the user storage
const UserStoreKey key = 10

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
				u.username
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
		&entry.AuthorUsername)
	if err != nil {
		return entry, err
	}

	var rows *sql.Rows
	rows, err = db.Query(`select t.name FROM teams AS t inner join entry_team ON t.id = entry_team.team_id where entry_team.entry_id = ?`, entryID)
	if err != nil {
		return entry, err
	}
	defer rows.Close()
	entry.Teams = make([]string, 0)

	for rows.Next() {
		var teamName string
		if err := rows.Scan(&teamName); err != nil {
			return entry, err
		}
		entry.Teams = append(entry.Teams, teamName)
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
			return ErrMissingEntryID
		}

		var entry, err = findEntryByID(db, payload.EntryID)
		if err != nil {
			return ErrEntryUnknown
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
			return ErrMissingTeamName
		}

		var team, err = findTeamByName(db, payload.TeamName)
		if err != nil {
			return ErrTeamUnknown
		}

		ctx = context.WithValue(ctx, TeamKey, &team)
		return h.ServeHTTPContext(ctx, rw, req)
	})
}

// Authenticated is a middleware that checks for authentication in the request
// Authentication is identified using the laravel_session cookie.
// If no authentication is present the request is halted.
func Authenticated(h ContextHandler) ContextHandler {
	return ContextHandlerFunc(func(ctx context.Context, rw http.ResponseWriter, req *http.Request) error {
		var db = ctx.Value(DBKey).(*sql.DB)

		var encryptedSessionID = ""
		for _, cookie := range req.Cookies() {
			if cookie.Name == "laravel_session" {
				encryptedSessionID = cookie.Value
			}
		}
		if encryptedSessionID == "" {
			return errors.New("Missing session cookie")
		}

		if sessionID, err := decrypt([]byte(encryptedSessionID)); err != nil {
			return err
		} else {
			if user, err := findUserByRememberToken(db, string(sessionID)); err != nil {
				return ErrAuthenticationRequired
			} else {
				ctx = context.WithValue(ctx, UserKey, &user)
				ctx = context.WithValue(ctx, UserStoreKey, &sqlUserStorage{db: db})
			}
		}

		return h.ServeHTTPContext(ctx, rw, req)
	})
}

// MaybeAuthenticated is a middleware that tries to authenticate a user by the session.
// If no authentication is present the request continues and the user is not set.
func MaybeAuthenticated(h ContextHandler) ContextHandler {
	return ContextHandlerFunc(func(ctx context.Context, rw http.ResponseWriter, req *http.Request) error {
		var db = ctx.Value(DBKey).(*sql.DB)

		var encryptedSessionID = ""
		for _, cookie := range req.Cookies() {
			if cookie.Name == "laravel_session" {
				encryptedSessionID = cookie.Value
			}
		}
		if encryptedSessionID != "" {
			if sessionID, err := decrypt([]byte(encryptedSessionID)); err == nil {
				if user, err := findUserByRememberToken(db, string(sessionID)); err == nil {
					ctx = context.WithValue(ctx, UserKey, &user)
				}
			}
		}

		return h.ServeHTTPContext(ctx, rw, req)
	})
}
