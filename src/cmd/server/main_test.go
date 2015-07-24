package main

import (
	"database/sql"
	"log"
	"os"
	"testing"

	"golang.org/x/net/context"

	_ "github.com/go-sql-driver/mysql"
)

func clearDatabase(db *sql.DB) {
	db.Exec(`DELETE FROM votes;`)
	db.Exec(`DELETE FROM team_user;`)
	db.Exec(`DELETE FROM entry_team;`)
	db.Exec(`DELETE FROM teams;`)
	db.Exec(`DELETE FROM identifiers;`)
	db.Exec(`DELETE FROM entries;`)
	db.Exec(`DELETE FROM users;`)
}

var (
	rootCtx context.Context
)

func TestMain(m *testing.M) {
	var db, err = sql.Open("mysql", "root@/dash3_test")
	if err != nil {
		log.Panicf("failed to connect to database")
	}
	defer db.Close()

	rootCtx = NewRootContext(db)
	clearDatabase(db)
	ret := m.Run()
	clearDatabase(db)

	os.Exit(ret)
}
