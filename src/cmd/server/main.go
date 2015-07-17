package main

import (
	"database/sql"
	"flag"
	"log"
	"net/http"
	"os"
	"time"

	"golang.org/x/net/context"

	entryStore "entry_storage"
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

type ContextAdapter struct {
	ctx     context.Context
	handler handlers.ContextHandler
}

func (ca *ContextAdapter) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	ca.handler.ServeHTTPContext(ca.ctx, rw, req)
}

func main() {
	var mux = http.DefaultServeMux

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

	var db, err = sql.Open(driverName, dataSource)
	if err != nil {
		log.Panicf("failed to connect to database")
	}
	defer db.Close()

	var userStorage = userStore.New(db)
	var entryStorage = entryStore.New(db)
	var voteStorage = voteStore.New(db)

	var rootContext = handlers.NewRootContext(db)

	mux.Handle("/users/register", http.StripPrefix("/users/register", &ContextAdapter{
		ctx:     rootContext,
		handler: handlers.ContextHandlerFunc(handlers.UsersRegister),
	}))
	mux.Handle("/users/login", http.StripPrefix("/users/login", &ContextAdapter{
		ctx:     rootContext,
		handler: handlers.ContextHandlerFunc(handlers.UserLogin),
	}))
	mux.Handle("/users/logout", http.StripPrefix("/users/logout", &ContextAdapter{
		ctx:     rootContext,
		handler: handlers.Authenticated(handlers.ContextHandlerFunc(handlers.UserLogout)),
	}))
	mux.Handle("/users/password", http.StripPrefix("/users/password", &ContextAdapter{
		ctx:     rootContext,
		handler: handlers.Authenticated(handlers.ContextHandlerFunc(handlers.UserChangePassword)),
	}))
	mux.Handle("/users/email", http.StripPrefix("/users/email", &ContextAdapter{
		ctx:     rootContext,
		handler: handlers.Authenticated(handlers.ContextHandlerFunc(handlers.UserChangeEmail)),
	}))
	// TODO(rr) add support for password forgotten requests /users/forgot/request
	// TODO(rr) add support for password reset requests /users/forgot/reset

	mux.Handle("/entries/", http.StripPrefix("/entries/", http.Handler(&handlers.EntriesHandler{
		UserStorage:  userStorage,
		EntryStorage: entryStorage,
		DB:           db,
		VoteStorage:  voteStorage,
	})))

	mux.Handle("/teams/list", http.StripPrefix("/teams/list", &ContextAdapter{
		ctx:     rootContext,
		handler: handlers.Authenticated(handlers.ContextHandlerFunc(handlers.TeamsList)),
	}))
	mux.Handle("/teams/create", http.StripPrefix("/teams/create", &ContextAdapter{
		ctx:     rootContext,
		handler: handlers.Authenticated(handlers.ContextHandlerFunc(handlers.TeamCreate)),
	}))
	mux.Handle("/teams/join", http.StripPrefix("/teams/join", &ContextAdapter{
		ctx:     rootContext,
		handler: handlers.Authenticated(handlers.WithTeam(handlers.ContextHandlerFunc(handlers.TeamJoin))),
	}))
	mux.Handle("/teams/leave", http.StripPrefix("/teams/leave", &ContextAdapter{
		ctx:     rootContext,
		handler: handlers.Authenticated(handlers.WithTeam(handlers.ContextHandlerFunc(handlers.TeamLeave))),
	}))
	mux.Handle("/teams/set_role", http.StripPrefix("/teams/set_role", &ContextAdapter{
		ctx:     rootContext,
		handler: handlers.Authenticated(handlers.WithTeam(handlers.ContextHandlerFunc(handlers.TeamSetRole))),
	}))
	mux.Handle("/teams/remove_member", http.StripPrefix("/teams/remove_member", &ContextAdapter{
		ctx:     rootContext,
		handler: handlers.Authenticated(handlers.WithTeam(handlers.ContextHandlerFunc(handlers.TeamRemoveMember))),
	}))
	mux.Handle("/teams/set_access_key", http.StripPrefix("/teams/set_access_key", &ContextAdapter{
		ctx:     rootContext,
		handler: handlers.Authenticated(handlers.WithTeam(handlers.ContextHandlerFunc(handlers.TeamSetAccessKey))),
	}))
	mux.Handle("/teams/list_members", http.StripPrefix("/teams/list_members", &ContextAdapter{
		ctx:     rootContext,
		handler: handlers.Authenticated(handlers.WithTeam(handlers.ContextHandlerFunc(handlers.TeamListMember))),
	}))

	var listen = ":8000"
	log.Printf("Listening on %q", listen)
	http.ListenAndServe(listen, logHandler(jsonHandler(mux)))
}
