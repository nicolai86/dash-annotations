package main

//go:generate go-bindata -pkg main -o bindata.go mysql/... sqlite3/...

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/mattn/go-sqlite3"
	"github.com/rubenv/sql-migrate"
)

func main() {
	migrate.SetTable("migrations")

	var (
		driverName string
		dataSource string
	)
	flag.StringVar(&driverName, "driver", "mysql", "database driver to use. currently mysql and sqlite3 are supported")
	flag.StringVar(&dataSource, "datasource", "", "datasource to be used with the database driver. mysql/sqlite3 DSN")

	flag.Parse()

	if dataSource == "" {
		log.Fatalf("missing data source! please re-run with --help for details")
		os.Exit(1)
	}

	migrations := &migrate.AssetMigrationSource{
		Asset:    Asset,
		AssetDir: AssetDir,
		Dir:      fmt.Sprintf("%s/migrations", driverName),
	}

	var db, err = sql.Open(driverName, dataSource)
	if err != nil {
		log.Panicf("failed to connect to database: %v", err)
	}
	defer db.Close()

	if _, err := migrate.Exec(db, driverName, migrations, migrate.Up); err != nil {
		log.Panicf("unable to migrate: %v", err)
	}
}
