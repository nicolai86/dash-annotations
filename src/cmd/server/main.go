package main

import (
	"database/sql"
	"log"
	"net/http"
	"time"

	entryStore "entry_storage"
	teamStore "team_storage"
	userStore "user_storage"
	voteStore "vote_storage"

	"handlers"

	_ "github.com/go-sql-driver/mysql"
)

func logHandler(next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf(
			"%s\t%s\t%s\t%s\t%s",
			r.RemoteAddr,
			time.Now().Format("2006-01-02T15:04:05 -0700"),
			r.Method,
			r.RequestURI,
			time.Since(start),
		)
	}
}

func jsonHandler(next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		next.ServeHTTP(w, r)
	}
}

func main() {
	var mux = http.DefaultServeMux

	var db, err = sql.Open("mysql", "root:@/dash3")
	if err != nil {
		log.Panicf("failed to connect to database")
	}
	var userStorage = userStore.New(db)
	var entryStorage = entryStore.New(db)
	var teamStorage = teamStore.New(db)
	var voteStorage = voteStore.New(db)

	// var entry, _ = entryStorage.FindByID(7)
	// fmt.Printf("%q", decorateBodyRendered(entry.BodyRendered))

	mux.Handle("/users/", http.StripPrefix("/users/", http.Handler(&handlers.UsersHandler{
		UserStorage: userStorage,
	})))
	mux.Handle("/entries/", http.StripPrefix("/entries/", http.Handler(&handlers.EntriesHandler{
		UserStorage:  userStorage,
		EntryStorage: entryStorage,
		TeamStorage:  teamStorage,
		VoteStorage:  voteStorage,
	})))
	mux.Handle("/teams/", http.StripPrefix("/teams/", http.Handler(&handlers.TeamsHandler{
		UserStorage: userStorage,
		TeamStorage: teamStorage,
	})))

	var listen = ":8000"
	log.Printf("Listening on %q", listen)
	http.ListenAndServe(listen, logHandler(jsonHandler(mux)))
}
