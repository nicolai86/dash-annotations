package main

//go:generate go-bindata -pkg main -o bindata.go templates/entries/

import (
	"database/sql"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/net/context"
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

// ContextAdapter is a compatability layer for http handlers which take a context
// as first arguments and return an error, to regular golang http handlers
type ContextAdapter struct {
	ctx     context.Context
	handler ContextHandler
}

// ServeHTTP implements the traditional golang net/http interface for a ContextAdapter
func (ca *ContextAdapter) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if err := ca.handler.ServeHTTPContext(ca.ctx, rw, req); err != nil {
		var enc = json.NewEncoder(rw)
		rw.WriteHeader(http.StatusBadRequest)
		enc.Encode(map[string]string{
			"status":  "error",
			"message": err.Error(),
		})
	}
}

func main() {
	var mux = http.DefaultServeMux

	var (
		driverName string
		dataSource string
		listen     string
	)
	flag.StringVar(&driverName, "driver", "mysql", "database driver to use. see github.com/rubenv/sql-migrate for details.")
	flag.StringVar(&dataSource, "datasource", "", "datasource to be used with the database driver. mysql/pg REVDSN")
	flag.StringVar(&listen, "listen", ":8000", "interface & port to listen on")
	flag.StringVar(&encryptionKey, "session.secret", "1234567812345678", "secret used to encrypt sessions. must have either 16, 24 or 32 bytes length")
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

	var userStorage = &sqlUserStorage{db: db}
	var rootContext = context.WithValue(NewRootContext(db), UserStoreKey, userStorage)

	mux.Handle("/users/register", &ContextAdapter{
		ctx:     rootContext,
		handler: ContextHandlerFunc(UserRegister),
	})
	mux.Handle("/users/login", &ContextAdapter{
		ctx:     rootContext,
		handler: ContextHandlerFunc(UserLogin),
	})
	mux.Handle("/users/logout", &ContextAdapter{
		ctx:     rootContext,
		handler: Authenticated(ContextHandlerFunc(UserLogout)),
	})
	mux.Handle("/users/password", &ContextAdapter{
		ctx:     rootContext,
		handler: Authenticated(ContextHandlerFunc(UserChangePassword)),
	})
	mux.Handle("/users/email", &ContextAdapter{
		ctx:     rootContext,
		handler: Authenticated(ContextHandlerFunc(UserChangeEmail)),
	})
	// TODO(rr) add support for password forgotten requests /users/forgot/request
	// TODO(rr) add support for password reset requests /users/forgot/reset

	mux.Handle("/entries/list", &ContextAdapter{
		ctx:     rootContext,
		handler: MaybeAuthenticated(ContextHandlerFunc(EntryList)),
	})
	mux.Handle("/entries/save", &ContextAdapter{
		ctx:     rootContext,
		handler: Authenticated(WithEntry(ContextHandlerFunc(EntrySave))),
	})
	mux.Handle("/entries/create", &ContextAdapter{
		ctx:     rootContext,
		handler: Authenticated(ContextHandlerFunc(EntryCreate)),
	})
	mux.Handle("/entries/get", &ContextAdapter{
		ctx:     rootContext,
		handler: MaybeAuthenticated(WithEntry(ContextHandlerFunc(EntryGet))),
	})
	mux.Handle("/entries/vote", &ContextAdapter{
		ctx:     rootContext,
		handler: Authenticated(WithEntry(ContextHandlerFunc(EntryVote))),
	})
	mux.Handle("/entries/delete", &ContextAdapter{
		ctx:     rootContext,
		handler: Authenticated(WithEntry(ContextHandlerFunc(EntryDelete))),
	})
	mux.Handle("/entries/remove_from_public", &ContextAdapter{
		ctx:     rootContext,
		handler: Authenticated(WithEntry(ContextHandlerFunc(EntryRemoveFromPublic))),
	})
	mux.Handle("/entries/remove_from_teams", &ContextAdapter{
		ctx:     rootContext,
		handler: Authenticated(WithEntry(ContextHandlerFunc(EntryRemoveFromTeams))),
	})

	mux.Handle("/teams/list", &ContextAdapter{
		ctx:     rootContext,
		handler: Authenticated(ContextHandlerFunc(TeamList)),
	})
	mux.Handle("/teams/create", &ContextAdapter{
		ctx:     rootContext,
		handler: Authenticated(ContextHandlerFunc(TeamCreate)),
	})
	mux.Handle("/teams/join", &ContextAdapter{
		ctx:     rootContext,
		handler: Authenticated(WithTeam(ContextHandlerFunc(TeamJoin))),
	})
	mux.Handle("/teams/leave", &ContextAdapter{
		ctx:     rootContext,
		handler: Authenticated(WithTeam(ContextHandlerFunc(TeamLeave))),
	})
	mux.Handle("/teams/set_role", &ContextAdapter{
		ctx:     rootContext,
		handler: Authenticated(WithTeam(ContextHandlerFunc(TeamSetRole))),
	})
	mux.Handle("/teams/remove_member", &ContextAdapter{
		ctx:     rootContext,
		handler: Authenticated(WithTeam(ContextHandlerFunc(TeamRemoveMember))),
	})
	mux.Handle("/teams/set_access_key", &ContextAdapter{
		ctx:     rootContext,
		handler: Authenticated(WithTeam(ContextHandlerFunc(TeamSetAccessKey))),
	})
	mux.Handle("/teams/list_members", &ContextAdapter{
		ctx:     rootContext,
		handler: Authenticated(WithTeam(ContextHandlerFunc(TeamListMember))),
	})

	log.Printf("Listening on %q", listen)
	http.ListenAndServe(listen, logHandler(jsonHandler(mux)))
}
