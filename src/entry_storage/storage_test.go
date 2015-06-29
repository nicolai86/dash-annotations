package storage

import (
	"database/sql"
	"log"
	"testing"

	"dash"

	_ "github.com/go-sql-driver/mysql"
)

var db *sql.DB

func setupDB() *sql.DB {
	var db, err = sql.Open("mysql", "root:@/dash3_test")
	if err != nil {
		log.Panicf("failed to connect to database: %v", err)
	}
	return db
}

func clearDB(db *sql.DB) {
	var tx, _ = db.Begin()
	tx.Exec(`SET FOREIGN_KEY_CHECKS = 0;`)
	tx.Exec(`truncate users;`)
	tx.Exec(`truncate teams;`)
	tx.Exec(`truncate team_entry;`)
	tx.Exec(`truncate team_user;`)
	tx.Exec(`truncate entries;`)
	tx.Exec(`truncate identifiers;`)
	tx.Exec(`SET FOREIGN_KEY_CHECKS = 1;`)
	tx.Commit()
}

func setupUser(db *sql.DB, username string) dash.User {
	var userID int64
	var res, err = db.Exec(`INSERT INTO users (username) VALUES (?)`, username)
	if err != nil {
		log.Fatalf("sql: %v", err)
	}
	userID, err = res.LastInsertId()
	if err != nil {
		log.Fatalf("sql: %v", err)
	}

	return dash.User{
		ID:       int(userID),
		Username: "nicolai86",
	}
}

func setupTeamAndMembership(db *sql.DB, team string, userID int) {
	var res, _ = db.Exec(`INSERT INTO teams (name) VALUES (?)`, team)
	var teamID, _ = res.LastInsertId()

	db.Exec(`INSERT INTO team_user (team_id, user_id, role) VALUES (?, ?, ?)`, teamID, userID, "owner")
}

func TestSQLStorage(t *testing.T) {
	var db = setupDB()
	clearDB(db)
	var stor = New(db)
	defer clearDB(db)

	var author = setupUser(db, "nicolai86")
	setupTeamAndMembership(db, "Test-Team", author.ID)

	var entry = dash.Entry{
		Title:  "a title",
		Body:   "a body",
		Public: true,
		Teams: []string{
			"Test-Team",
		},
		Type:   "Comment",
		Anchor: "barf",
		Identifier: dash.IdentifierDict{
			DocsetName:     "Go",
			DocsetFilename: "Go",
			DocsetPlatform: "go",
			DocsetBundle:   "go",
			DocsetVersion:  "1",
			PagePath:       "golang.org/pkg/net/http/index.html",
			PageTitle:      "http",
			HttrackSource:  "golang.org/pkg/net/http/",
		},
	}

	if err := stor.Store(&entry, author); err != nil {
		t.Fatalf("failed to store: %v", err)
	}

	var entryTeamCnt int64
	db.QueryRow(`SELECT count(*) FROM entry_team`).Scan(&entryTeamCnt)
	if entryTeamCnt != 1 {
		t.Fatalf("Expected entry <-> team relationship to be set up")
	}

	if entry.ID == 0 {
		t.Fatalf("failed to set id")
	}
	if entry.IdentifierID == 0 {
		t.Fatalf("failed to set identifier id")
	}

	if _, err := stor.FindByID(entry.ID); err != nil {
		t.Fatalf("failed to reload: %v", err)
	}

	var foreign = setupUser(db, "hummer")
	if entries, err := stor.FindOwnByIdentifier(entry.Identifier, &foreign); len(entries) != 0 || err != nil {
		t.Fatalf("length is wrong: %v %v", entries, err)
	}

	if entries, err := stor.FindOwnByIdentifier(entry.Identifier, &author); len(entries) != 1 || err != nil {
		t.Fatalf("length is wrong: %v %v", entries, err)
	}

	if entries, err := stor.FindPublicByIdentifier(entry.Identifier, &foreign); len(entries) != 1 || err != nil {
		t.Fatalf("length is wrong: %v %v", entries, err)
	}

	if err := stor.Delete(&entry); err != nil {
		t.Fatalf("expected delete to go through")
	}
}
