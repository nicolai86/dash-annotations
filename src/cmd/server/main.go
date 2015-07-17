package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
	"time"

	"golang.org/x/net/context"

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

// ContextAdapter is a compatability layer for http handlers which take a context
// as first arguments and return an error, to regular golang http handlers
type ContextAdapter struct {
	ctx     context.Context
	handler handlers.ContextHandler
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

	var rootContext = handlers.NewRootContext(db)

	mux.Handle("/users/register", &ContextAdapter{
		ctx:     rootContext,
		handler: handlers.ContextHandlerFunc(handlers.UsersRegister),
	})
	mux.Handle("/users/login", &ContextAdapter{
		ctx:     rootContext,
		handler: handlers.ContextHandlerFunc(handlers.UserLogin),
	})
	mux.Handle("/users/logout", &ContextAdapter{
		ctx:     rootContext,
		handler: handlers.Authenticated(handlers.ContextHandlerFunc(handlers.UserLogout)),
	})
	mux.Handle("/users/password", &ContextAdapter{
		ctx:     rootContext,
		handler: handlers.Authenticated(handlers.ContextHandlerFunc(handlers.UserChangePassword)),
	})
	mux.Handle("/users/email", &ContextAdapter{
		ctx:     rootContext,
		handler: handlers.Authenticated(handlers.ContextHandlerFunc(handlers.UserChangeEmail)),
	})
	// TODO(rr) add support for password forgotten requests /users/forgot/request
	// TODO(rr) add support for password reset requests /users/forgot/reset

	mux.Handle("/entries/list", &ContextAdapter{
		ctx:     rootContext,
		handler: handlers.MaybeAuthenticated(handlers.ContextHandlerFunc(handlers.EntriesList)),
	})
	mux.Handle("/entries/save", &ContextAdapter{
		ctx:     rootContext,
		handler: handlers.Authenticated(handlers.ContextHandlerFunc(handlers.EntriesSave)),
	})
	mux.Handle("/entries/create", &ContextAdapter{
		ctx:     rootContext,
		handler: handlers.Authenticated(handlers.ContextHandlerFunc(handlers.EntriesSave)),
	})
	mux.Handle("/entries/get", &ContextAdapter{
		ctx:     rootContext,
		handler: handlers.MaybeAuthenticated(handlers.WithEntry(handlers.ContextHandlerFunc(handlers.EntryGet))),
	})
	mux.Handle("/entries/vote", &ContextAdapter{
		ctx:     rootContext,
		handler: handlers.Authenticated(handlers.WithEntry(handlers.ContextHandlerFunc(handlers.EntryVote))),
	})
	mux.Handle("/entries/delete", &ContextAdapter{
		ctx:     rootContext,
		handler: handlers.Authenticated(handlers.WithEntry(handlers.ContextHandlerFunc(handlers.EntryDelete))),
	})
	mux.Handle("/entries/remove_from_public", &ContextAdapter{
		ctx:     rootContext,
		handler: handlers.Authenticated(handlers.WithEntry(handlers.ContextHandlerFunc(handlers.EntryRemoveFromPublic))),
	})
	mux.Handle("/entries/remove_from_teams", &ContextAdapter{
		ctx:     rootContext,
		handler: handlers.Authenticated(handlers.WithEntry(handlers.ContextHandlerFunc(handlers.EntryRemoveFromTeams))),
	})

	mux.Handle("/teams/list", &ContextAdapter{
		ctx:     rootContext,
		handler: handlers.Authenticated(handlers.ContextHandlerFunc(handlers.TeamsList)),
	})
	mux.Handle("/teams/create", &ContextAdapter{
		ctx:     rootContext,
		handler: handlers.Authenticated(handlers.ContextHandlerFunc(handlers.TeamCreate)),
	})
	mux.Handle("/teams/join", &ContextAdapter{
		ctx:     rootContext,
		handler: handlers.Authenticated(handlers.WithTeam(handlers.ContextHandlerFunc(handlers.TeamJoin))),
	})
	mux.Handle("/teams/leave", &ContextAdapter{
		ctx:     rootContext,
		handler: handlers.Authenticated(handlers.WithTeam(handlers.ContextHandlerFunc(handlers.TeamLeave))),
	})
	mux.Handle("/teams/set_role", &ContextAdapter{
		ctx:     rootContext,
		handler: handlers.Authenticated(handlers.WithTeam(handlers.ContextHandlerFunc(handlers.TeamSetRole))),
	})
	mux.Handle("/teams/remove_member", &ContextAdapter{
		ctx:     rootContext,
		handler: handlers.Authenticated(handlers.WithTeam(handlers.ContextHandlerFunc(handlers.TeamRemoveMember))),
	})
	mux.Handle("/teams/set_access_key", &ContextAdapter{
		ctx:     rootContext,
		handler: handlers.Authenticated(handlers.WithTeam(handlers.ContextHandlerFunc(handlers.TeamSetAccessKey))),
	})
	mux.Handle("/teams/list_members", &ContextAdapter{
		ctx:     rootContext,
		handler: handlers.Authenticated(handlers.WithTeam(handlers.ContextHandlerFunc(handlers.TeamListMember))),
	})

	log.Printf("Listening on %q", listen)
	http.ListenAndServe(listen, logHandler(jsonHandler(mux)))
}
