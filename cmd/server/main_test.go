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
	db      *sql.DB
)

func TestMain(m *testing.M) {
	var driver = os.Getenv("TEST_DRIVER")
	if driver == "" {
		driver = "mysql"
	}
	var datasource = os.Getenv("TEST_DATASOURCE")
	if datasource == "" {
		datasource = "root@/dash3_test"
	}

	log.Printf("test using driver %q with source %q\n", driver, datasource)
	var err error
	db, err = sql.Open(driver, datasource)
	if err != nil {
		log.Panicf("failed to connect to database")
	}
	defer db.Close()
	if err := runMigrations(db, driver); err != nil {
		panic(err)
	}

	rootCtx = NewRootContext(db)
	clearDatabase(db)
	ret := m.Run()
	clearDatabase(db)

	os.Exit(ret)
}
