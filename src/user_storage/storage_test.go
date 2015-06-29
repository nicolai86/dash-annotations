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
	db.Exec(`DELETE FROM users;`)
}

func TestSQLStorage(t *testing.T) {
	var db = setupDB()
	var stor = New(db)
	defer clearDB(db)

	stor.Store(&dash.User{
		Username: "nicolai86-2",
	})

	var u = dash.User{
		Username: "nicolai86",
	}
	// create
	u.ChangePassword("quark")
	var previousUserID = u.ID
	if err := stor.Store(&u); err != nil {
		t.Fatalf("failed to store user: %v", err)
	}
	if u.ID == previousUserID {
		t.Fatalf("failed to set user id")
	}

	if !u.PasswordsMatch("quark") {
		t.Fatalf("expected passwords to match")
	}

	// find by username
	if _, err := stor.FindByUsername("nicolai87"); err == nil {
		t.Fatalf("nicolai87 does not exist")
	}
	if u2, err := stor.FindByUsername("nicolai86"); err != nil {
		t.Fatalf("nicolai86 should exist")
	} else {
		if !u2.PasswordsMatch("quark") {
			t.Fatalf("expected passwords to match")
		}
		if u2.ID != u.ID {
			t.Fatalf("different user!!")
		}
	}

	// update
	u.ChangePassword("barf2")
	u.RememberToken = sql.NullString{String: "rember", Valid: true}
	u.Email = sql.NullString{String: "nicolai86@me.com", Valid: true}
	if err := stor.Update(&u); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	if u2, err := stor.FindByUsername("nicolai86"); err != nil {
		t.Fatalf("nicolai86 should exist")
	} else {
		if !u2.RememberToken.Valid || u2.RememberToken.String != "rember" {
			t.Fatalf("failed to update remember token")
		}
		if !u2.Email.Valid || u2.Email.String != "nicolai86@me.com" {
			t.Fatalf("failed to update email")
		}
		if !u2.PasswordsMatch("barf2") {
			t.Fatalf("expected passwords to match")
		}
	}

	u.RememberToken = sql.NullString{Valid: true}
	stor.Update(&u)

	if u2, err := stor.FindByUsername("nicolai86"); err != nil {
		t.Fatalf("nicolai86 should exist")
	} else {
		if u2.RememberToken.String != "" {
			t.Fatalf("failed to update remember token: %v", u2.RememberToken)
		}
	}
}
