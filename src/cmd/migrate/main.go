package main

//go:generate go-bindata -pkg main -o bindata.go migrations/

import (
	"database/sql"
	"flag"
	"log"
	"os"

	_ "github.com/go-sql-driver/mysql"
	"github.com/rubenv/sql-migrate"
)

func main() {
	migrate.SetTable("migrations")

	var (
		driverName string
		dataSource string
	)
	flag.StringVar(&driverName, "driver", "mysql", "database driver to use. see github.com/rubenv/sql-migrate for details.")
	flag.StringVar(&dataSource, "datasource", "", "datasource to be used with the database driver. mysql/pg REVDSN")

	flag.Parse()

	if dataSource == "" {
		log.Fatalf("missing data source! please re-run with --help for details")
		os.Exit(1)
	}

	migrations := &migrate.AssetMigrationSource{
		Asset:    Asset,
		AssetDir: AssetDir,
		Dir:      "migrations",
	}

	var db, err = sql.Open(driverName, dataSource)
	if err != nil {
		log.Panicf("failed to connect to database: %v", err)
	}
	defer db.Close()

	if _, err := migrate.Exec(db, "mysql", migrations, migrate.Up); err != nil {
		log.Panicf("unable to migrate: %v", err)
	}
}
