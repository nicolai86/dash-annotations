package handlers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"text/template"

	"golang.org/x/net/context"

	entryStore "entry_storage"
	userStore "user_storage"
	voteStore "vote_storage"

	"dash"
)

func findTeamMembershipsForUser(db *sql.DB, u dash.User) ([]dash.TeamMember, error) {
	var rows, err = db.Query(`SELECT t.id, t.name, tm.role FROM team_user AS tm INNER JOIN teams AS t ON t.id = tm.team_id WHERE tm.user_id = ?`, u.ID)
	defer rows.Close()
	if err != nil {
		return nil, err
	}

	var memberships = make([]dash.TeamMember, 0)
	for rows.Next() {
		var membership = dash.TeamMember{}
		if err := rows.Scan(&membership.TeamID, &membership.TeamName, &membership.Role); err != nil {
			return nil, err
		}
		memberships = append(memberships, membership)
	}

	return memberships, nil
}

type EntriesHandler struct {
	UserStorage  userStore.Storage
	EntryStorage entryStore.Storage
	VoteStorage  voteStore.Storage
	DB           *sql.DB
}

type entriesListRequest struct {
	Identifier dash.IdentifierDict `json:"identifier"`
}
type entriesListResponse struct {
	Status        string       `json:"status"`
	PublicEntries []dash.Entry `json:"public_entries,omitempty"`
	OwnEntries    []dash.Entry `json:"own_entries,omitempty"`
	TeamEntries   []dash.Entry `json:"team_entries,omitempty"`
}

func EntriesList(ctx context.Context, w http.ResponseWriter, req *http.Request) {
	var db = ctx.Value(DBKey).(*sql.DB)
	var user *dash.User = nil
	if ctx.Value(UserKey) != nil {
		user = ctx.Value(UserKey).(*dash.User)
	}

	if user != nil {
		var teams, _ = findTeamMembershipsForUser(db, *user)
		user.TeamMemberships = teams
	}

	var dec = json.NewDecoder(req.Body)
	var listReq entriesListRequest
	dec.Decode(&listReq)
	var enc = json.NewEncoder(w)

	var entryStorage = entryStore.New(db)
	var public, _ = entryStorage.FindPublicByIdentifier(listReq.Identifier, user)
	var own, _ = entryStorage.FindOwnByIdentifier(listReq.Identifier, user)
	var team []dash.Entry = nil
	if user != nil {
		team, _ = entryStorage.FindByTeamAndIdentifier(listReq.Identifier, *user)
	}
	// TODO remove from public which are in team

	var resp = entriesListResponse{
		Status:        "success",
		PublicEntries: public,
		OwnEntries:    own,
		TeamEntries:   team,
	}
	enc.Encode(resp)
}

type entrySaveRequest struct {
	Title          string              `json:"title"`
	Body           string              `json:"body"`
	Public         bool                `json:"public"`
	Type           string              `json:"type"`
	Teams          []string            `json:"teams"`
	License        string              `json:"license"`
	IdentifierDict dash.IdentifierDict `json:"identifier"`
	Anchor         string              `json:"anchor"`
	EntryID        int                 `json:"entry_id"`
}

type entrySaveResponse struct {
	Status string     `json:"status"`
	Entry  dash.Entry `json:"entry"`
}

func EntriesSave(ctx context.Context, w http.ResponseWriter, req *http.Request) {
	var db = ctx.Value(DBKey).(*sql.DB)
	var user = ctx.Value(UserKey).(*dash.User)
	var enc = json.NewEncoder(w)
	var dec = json.NewDecoder(req.Body)
	var payload entrySaveRequest
	dec.Decode(&payload)

	var entryStorage = entryStore.New(db)

	var entry = dash.Entry{
		ID:         payload.EntryID,
		Title:      payload.Title,
		Type:       payload.Type,
		Body:       payload.Body,
		Public:     payload.Public,
		Identifier: payload.IdentifierDict,
		Anchor:     payload.Anchor,
		Teams:      payload.Teams,
	}
	if err := entryStorage.Store(&entry, *user); err != nil {
		w.WriteHeader(http.StatusBadRequest)

		switch err {
		case entryStore.ErrMissingTitle, entryStore.ErrMissingBody, entryStore.ErrMissingAnchor:
			log.Printf("invalid entry/save")
			enc.Encode(map[string]string{
				"status":  "error",
				"message": "Oops. Unknown error",
			})
		case entryStore.ErrPublicAnnotationForbidden:
			log.Printf("public annotation forbidden")
			enc.Encode(map[string]string{
				"status":  "error",
				"message": "Public annotations are not allowed on this page",
			})
		case entryStore.ErrUpdateForbidden:
			log.Printf("update forbidden")
			enc.Encode(map[string]string{
				"status":  "error",
				"message": "Error. Logout and try again",
			})
		default:
			log.Printf("Unknown error: %v", err)
			enc.Encode(map[string]string{
				"status":  "error",
				"message": "Oops. Unknown error",
			})
		}
		return
	}

	var vote = dash.Vote{
		EntryID: entry.ID,
		UserID:  user.ID,
		Type:    dash.VoteUp,
	}
	var voteStorage = voteStore.New(db)
	voteStorage.Upsert(&vote)
	entryStorage.UpdateScore(&entry)

	var resp = entrySaveResponse{
		Entry:  entry,
		Status: "success",
	}
	enc.Encode(resp)
}

type entryGetRequest struct {
	EntryID int `json:"entry_id"`
}
type entryGetResponse struct {
	Status          string `json:"status"`
	Body            string `json:"body"`
	BodyRendered    string `json:"body_rendered"`
	GlobalModerator bool   `json:"global_moderator"`
}

type decoratedContext struct {
	Entry dash.Entry
	User  dash.User
	Vote  dash.Vote
}

func decorateBodyRendered(entry dash.Entry, user dash.User, vote dash.Vote) string {
	var f, _ = os.Open("./templates/entries/get.html")
	defer f.Close()
	var rawTmpl, _ = ioutil.ReadAll(f)
	var html *template.Template
	var err error

	var fns = template.FuncMap{
		"join": strings.Join,
		"max": func(upper, current int) int {
			if current > upper {
				return upper
			}
			return current
		},
		"min": func(lower, current int) int {
			if current < lower {
				return lower
			}
			return current
		},
		"surroundOwnTeamWith": func(elem string, ss []string) []string {
			var resp = make([]string, 0)
			for _, str := range ss {
				var ownTeam = false
				for _, membership := range user.TeamMemberships {
					ownTeam = ownTeam || membership.TeamName == str
				}
				if ownTeam {
					resp = append(resp, "<"+elem+">"+str+"</"+elem+">")
				} else {
					resp = append(resp, str)
				}
			}
			return resp
		},
		"isTeamModerator": func(user dash.User, teams []string) bool {
			var isModerator = false
			for _, membership := range user.TeamMemberships {
				for _, team := range teams {
					isModerator = isModerator ||
						(team == membership.TeamName && (membership.Role == "owner" || membership.Role == "moderator"))
				}
			}
			return isModerator
		},
	}
	html, err = template.New("get.html").Funcs(fns).Parse(string(rawTmpl))
	if err != nil {
		log.Panic(err)
	}
	var tmp = bytes.Buffer{}
	var c = decoratedContext{
		Entry: entry,
		User:  user,
		Vote:  vote,
	}

	err = html.Execute(&tmp, &c)
	var dd, _ = ioutil.ReadAll(&tmp)
	return string(dd)
}

func EntryGet(ctx context.Context, w http.ResponseWriter, req *http.Request) {
	var db = ctx.Value(DBKey).(*sql.DB)
	var user = dash.User{}
	if ctx.Value(UserKey) != nil {
		user = dash.User(*ctx.Value(UserKey).(*dash.User))
	}

	var entry = ctx.Value(EntryKey).(*dash.Entry)
	var enc = json.NewEncoder(w)
	var voteStorage = voteStore.New(db)
	var vote, _ = voteStorage.FindVoteByEntryAndUser(*entry, user)
	var teams, _ = findTeamMembershipsForUser(db, user)
	user.TeamMemberships = teams
	var resp = entryGetResponse{
		Status:          "success",
		Body:            entry.Body,
		BodyRendered:    decorateBodyRendered(*entry, user, vote),
		GlobalModerator: false,
	}
	enc.Encode(resp)
}

type voteRequest struct {
	VoteType int `json:"vote_type"`
	EntryID  int `json:"entry_id"`
}

func EntryVote(ctx context.Context, w http.ResponseWriter, req *http.Request) {
	var db = ctx.Value(DBKey).(*sql.DB)
	var user = ctx.Value(UserKey).(*dash.User)

	var enc = json.NewEncoder(w)
	var payload voteRequest
	var dec = json.NewDecoder(req.Body)
	dec.Decode(&payload)

	var entry = ctx.Value(EntryKey).(*dash.Entry)
	var entryStorage = entryStore.New(db)
	var vote dash.Vote
	var voteStorage = voteStore.New(db)
	vote, _ = voteStorage.FindVoteByEntryAndUser(*entry, *user)
	vote.Type = payload.VoteType
	voteStorage.Upsert(&vote)
	entryStorage.UpdateScore(entry)
	enc.Encode(map[string]string{
		"status": "success",
	})
}

func EntryDelete(ctx context.Context, w http.ResponseWriter, req *http.Request) {
	var db = ctx.Value(DBKey).(*sql.DB)
	var user = ctx.Value(UserKey).(*dash.User)

	var enc = json.NewEncoder(w)
	var payload entryGetRequest
	var dec = json.NewDecoder(req.Body)
	dec.Decode(&payload)

	var entry = ctx.Value(EntryKey).(*dash.Entry)
	var entryStorage = entryStore.New(db)
	if entry.UserID != user.ID {
		enc.Encode(map[string]string{
			"status":  "error",
			"message": "Error. Logout and try again",
		})
		return
	}

	entryStorage.Delete(entry)

	enc.Encode(map[string]string{
		"status": "success",
	})
}

func EntryRemoveFromPublic(ctx context.Context, w http.ResponseWriter, req *http.Request) {
	var db = ctx.Value(DBKey).(*sql.DB)
	var user = ctx.Value(UserKey).(*dash.User)

	var enc = json.NewEncoder(w)
	var payload entryGetRequest
	var dec = json.NewDecoder(req.Body)
	dec.Decode(&payload)

	var entry = ctx.Value(EntryKey).(*dash.Entry)
	var entryStorage = entryStore.New(db)
	entry.RemovedFromPublic = true
	if err := entryStorage.Store(entry, *user); err != nil {
		log.Printf("fuck: %v", err)
		enc.Encode(map[string]string{
			"status":  "error",
			"message": "Oops. Unknown error",
		})
		return
	}

	enc.Encode(map[string]string{
		"status": "success",
	})
}

func EntryRemoveFromTeams(ctx context.Context, w http.ResponseWriter, req *http.Request) {
	var db = ctx.Value(DBKey).(*sql.DB)
	var user = ctx.Value(UserKey).(*dash.User)

	var enc = json.NewEncoder(w)
	var payload entryGetRequest
	var dec = json.NewDecoder(req.Body)
	dec.Decode(&payload)

	var entry = ctx.Value(EntryKey).(*dash.Entry)
	var entryStorage = entryStore.New(db)
	var teams, _ = findTeamMembershipsForUser(db, *user)
	user.TeamMemberships = teams

	var isTeamModerator = false
	for _, membership := range user.TeamMemberships {
		for _, team := range entry.Teams {
			isTeamModerator = isTeamModerator ||
				(team == membership.TeamName && (membership.Role == "owner" || membership.Role == "moderator"))
		}
	}
	var err error
	if err != nil || !isTeamModerator {
		log.Printf("You are no team moderator")
		enc.Encode(map[string]string{
			"status":  "error",
			"message": "Error. Logout and try again",
		})
		return
	}

	if err := entryStorage.RemoveFromTeams(*entry, *user); err != nil {
		log.Printf("error removing from teams: %v", err)
		enc.Encode(map[string]string{
			"status":  "error",
			"message": "Oops. Unknown error",
		})
		return
	}

	enc.Encode(map[string]string{
		"status": "success",
	})
}
