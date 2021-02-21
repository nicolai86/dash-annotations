package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nicolai86/dash-annotations/dash"
)

func exec(query string, params ...interface{}) int {
	var res, err = db.Exec(query, params...)
	if err != nil {
		panic(err)
	}
	var id int64
	id, _ = res.LastInsertId()
	return int(id)
}

func TestWithEntry_Success(t *testing.T) {
	var userID = exec(`INSERT INTO users (username, password) VALUES (?, ?)`, "a2", "b2")
	var identifierID = exec(`INSERT INTO identifiers (docset_name, docset_filename, docset_platform, docset_bundle, docset_version, page_path, page_title, httrack_source, banned_from_public) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, "a", "b", "c", "d", "e", "f", "g", "h", false)
	var entryID = exec(`INSERT INTO entries (title, body, body_rendered, type, identifier_id, anchor, user_id, public, removed_from_public, score) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, "a", "b", "b", "comment", identifierID, "c", userID, true, false, 1)

	req, _ := http.NewRequest("POST", "/dont-care", strings.NewReader(fmt.Sprintf(`{"entry_id":%d}`, entryID)))
	rw := httptest.NewRecorder()

	var err = WithEntry(ContextHandlerFunc(func(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
		var entry, ok = ctx.Value(EntryKey).(*dash.Entry)
		if !ok {
			t.Fatalf("Expected WithEntry to include a entry")
		}
		if entry.ID != int(entryID) {
			t.Fatalf("Expected WithEntry to include entry %q, got %q", entry.ID, entryID)
		}
		return nil
	})).ServeHTTPContext(rootCtx, rw, req)

	if err != nil {
		t.Fatalf("Expected WithEntry not to return an error, got %q", err)
	}
}

func TestWithEntry_UnknownEntry(t *testing.T) {
	req, _ := http.NewRequest("POST", "/dont-care", strings.NewReader(`{"entry_id":231233333123}`))
	rw := httptest.NewRecorder()

	var err = WithEntry(ContextHandlerFunc(func(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
		t.Fatalf("WithEntry should halt the request")
		return nil
	})).ServeHTTPContext(rootCtx, rw, req)

	if err != ErrEntryUnknown {
		t.Fatalf("Expected WithEntry not to return an error, got %q", err)
	}
}

func TestWithEntry_MissingEntryID(t *testing.T) {
	req, _ := http.NewRequest("POST", "/dont-care", strings.NewReader(`{"entry_id":null}`))
	rw := httptest.NewRecorder()

	var err = WithEntry(ContextHandlerFunc(func(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
		t.Fatalf("WithEntry should halt the request")
		return nil
	})).ServeHTTPContext(rootCtx, rw, req)

	if err != ErrMissingEntryID {
		t.Fatalf("Expected WithEntry not to return an error, got %q", err)
	}
}

func TestWithTeam_Success(t *testing.T) {
	var teamID = exec(`INSERT INTO teams (name) VALUES (?)`, "team")
	var userID = exec(`INSERT INTO users (username, password) VALUES (?, ?)`, "a", "b")
	db.Exec(`INSERT INTO team_user (team_id, user_id, role) VALUES ( ?, ?, ? )`, teamID, userID, "owner")

	req, _ := http.NewRequest("POST", "/dont-care", strings.NewReader(`{"name":"team"}`))
	rw := httptest.NewRecorder()

	var err = WithTeam(ContextHandlerFunc(func(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
		var team, ok = ctx.Value(TeamKey).(*dash.Team)
		if !ok {
			t.Fatalf("Expected WithTeam to include a team")
		}
		if team.Name != "team" {
			t.Fatalf("Expectect WithTeam to extract %q but got %q", "team", team.Name)
		}
		return nil
	})).ServeHTTPContext(rootCtx, rw, req)

	if err != nil {
		t.Fatalf("Expected WithTeam not to return an error, got %q", err)
	}
}

func TestWithTeam_UnknownTeam(t *testing.T) {
	req, _ := http.NewRequest("POST", "/dont-care", strings.NewReader(`{"name":"unknown-team"}`))
	rw := httptest.NewRecorder()

	var err = WithTeam(ContextHandlerFunc(func(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
		t.Fatalf("Expected WithTeam to halt the request")
		return nil
	})).ServeHTTPContext(rootCtx, rw, req)

	if err != ErrTeamUnknown {
		t.Fatalf("Expected WithTeam to return %q, got %q", ErrTeamUnknown, err)
	}
}

func TestWithTeam_MissingTeamName(t *testing.T) {
	req, _ := http.NewRequest("POST", "/dont-care", strings.NewReader(`{"name":""}`))
	rw := httptest.NewRecorder()

	var err = WithTeam(ContextHandlerFunc(func(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
		t.Fatalf("Expected WithTeam to halt the request")
		return nil
	})).ServeHTTPContext(rootCtx, rw, req)

	if err != ErrMissingTeamName {
		t.Fatalf("Expected WithTeam to return %q, got %q", ErrMissingTeamName, err)
	}
}

func TestEncryptDecrypt(t *testing.T) {
	var encrypted, decrypted []byte
	var err error
	var rand string
	rand, err = generateRandomString(10)
	encrypted, err = encrypt([]byte(rand))
	if err != nil {
		t.Fatalf("Encryption failed: %v", err)
	}

	decrypted, err = decrypt(encrypted)
	if err != nil {
		t.Fatalf("Decryption failed: %v", err)
	}

	if string(rand) != string(decrypted) {
		t.Fatalf("Expected %q to eql %q", string(decrypted), string(rand))
	}
}

func TestAuthenticated_Success(t *testing.T) {
	t.Parallel()
	db.Exec(`INSERT INTO users (username, password, remember_token) VALUES (?, ?, ?)`, "test", "ddd", "asd")

	req, _ := http.NewRequest("POST", "/dont-care", strings.NewReader(``))

	encryptedSessionID, _ := encrypt([]byte("asd"))
	req.AddCookie(&http.Cookie{
		Name:  "laravel_session",
		Value: string(encryptedSessionID),
	})

	rw := httptest.NewRecorder()

	var err = Authenticated(ContextHandlerFunc(func(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
		var user, ok = ctx.Value(UserKey).(*dash.User)
		if !ok {
			t.Fatalf("Expected Authenticated to include a user")
		}
		if user.Username != "test" {
			t.Fatalf("Expectect Authenticated to extract %q but got %q", "test", user.Username)
		}
		return nil
	})).ServeHTTPContext(rootCtx, rw, req)

	if err != nil {
		t.Fatalf("Expected Authenticated not to return an error, got %q", err)
	}
}

func TestAuthenticated_Failure(t *testing.T) {
	t.Parallel()
	db.Exec(`INSERT INTO users (username, password, remember_token) VALUES (?, ?, ?)`, "test", "ddd", "won'tmatch")

	req, _ := http.NewRequest("POST", "/dont-care", strings.NewReader(``))

	encryptedSessionID, _ := encrypt([]byte("wontmatch"))
	req.AddCookie(&http.Cookie{
		Name:  "laravel_session",
		Value: string(encryptedSessionID),
	})

	rw := httptest.NewRecorder()

	var err = Authenticated(ContextHandlerFunc(func(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
		t.Fatalf("Expected authenticated to halt the request")
		return nil
	})).ServeHTTPContext(rootCtx, rw, req)

	if err != ErrAuthenticationRequired {
		t.Fatalf("Expected Authenticated to return ErrAuthenticationRequired, got %q", err)
	}
}

func TestMaybeAuthenticated_Success(t *testing.T) {
	t.Parallel()
	db.Exec(`INSERT INTO users (username, password, remember_token) VALUES (?, ?, ?)`, "test", "ddd", "asd")

	req, _ := http.NewRequest("POST", "/dont-care", strings.NewReader(``))

	encryptedSessionID, _ := encrypt([]byte("asd"))
	req.AddCookie(&http.Cookie{
		Name:  "laravel_session",
		Value: string(encryptedSessionID),
	})

	rw := httptest.NewRecorder()

	var err = MaybeAuthenticated(ContextHandlerFunc(func(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
		var user, ok = ctx.Value(UserKey).(*dash.User)
		if !ok {
			t.Fatalf("Expected MaybeAuthenticated to include a user")
		}
		if user.Username != "test" {
			t.Fatalf("Expectect MaybeAuthenticated to extract %q but got %q", "test", user.Username)
		}
		return nil
	})).ServeHTTPContext(rootCtx, rw, req)

	if err != nil {
		t.Fatalf("Expected MaybeAuthenticated not to return an error, got %q", err)
	}
}

func TestMaybeAuthenticated_Failure(t *testing.T) {
	t.Parallel()
	db.Exec(`INSERT INTO users (username, password, remember_token) VALUES (?, ?, ?)`, "test", "ddd", "won'tmatch")

	req, _ := http.NewRequest("POST", "/dont-care", strings.NewReader(``))

	encryptedSessionID, _ := encrypt([]byte("wontmatch"))
	req.AddCookie(&http.Cookie{
		Name:  "laravel_session",
		Value: string(encryptedSessionID),
	})

	rw := httptest.NewRecorder()

	var err = MaybeAuthenticated(ContextHandlerFunc(func(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
		var _, ok = ctx.Value(UserKey).(*dash.User)
		if ok {
			t.Fatalf("Expected MaybeAuthenticated not to include a user")
		}
		return nil
	})).ServeHTTPContext(rootCtx, rw, req)

	if err != nil {
		t.Fatalf("Expected MaybeAuthenticated not to return an error, got %q", err)
	}
}
