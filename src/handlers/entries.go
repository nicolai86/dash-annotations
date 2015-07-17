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

func (eh *EntriesHandler) list(w http.ResponseWriter, req *http.Request) {
	var u, err = getUserFromSession(eh.UserStorage, req)
	var u2 *dash.User = &u
	if err != nil {
		u2 = nil
	} else {
		var teams, _ = findTeamMembershipsForUser(eh.DB, u)
		u.TeamMemberships = teams
	}
	var dec = json.NewDecoder(req.Body)
	var listReq entriesListRequest
	dec.Decode(&listReq)
	var enc = json.NewEncoder(w)
	var public, _ = eh.EntryStorage.FindPublicByIdentifier(listReq.Identifier, u2)
	var own, _ = eh.EntryStorage.FindOwnByIdentifier(listReq.Identifier, u2)
	var team, _ = eh.EntryStorage.FindByTeamAndIdentifier(listReq.Identifier, u)

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

func (eh *EntriesHandler) save(w http.ResponseWriter, req *http.Request) {
	var u, err = getUserFromSession(eh.UserStorage, req)

	var enc = json.NewEncoder(w)
	if err != nil {
		log.Printf("not logged in.")
		enc.Encode(map[string]string{
			"status":  "error",
			"message": "Error. Logout and try again",
		})
		return
	}

	var dec = json.NewDecoder(req.Body)
	var payload entrySaveRequest
	dec.Decode(&payload)

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
	if err := eh.EntryStorage.Store(&entry, u); err != nil {
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
		UserID:  u.ID,
		Type:    dash.VoteUp,
	}
	eh.VoteStorage.Upsert(&vote)
	eh.EntryStorage.UpdateScore(&entry)

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

func (eh *EntriesHandler) get(w http.ResponseWriter, req *http.Request) {
	var dec = json.NewDecoder(req.Body)
	var payload entryGetRequest
	dec.Decode(&payload)
	var enc = json.NewEncoder(w)
	var entry, err = eh.EntryStorage.FindByID(payload.EntryID)
	if err != nil {
		log.Printf("Unknown Entry! %v", err)
		enc.Encode(map[string]string{
			"status":  "error",
			"message": "Oops. Unknown error",
		})
		return
	}
	var u, _ = getUserFromSession(eh.UserStorage, req)
	var vote, _ = eh.VoteStorage.FindVoteByEntryAndUser(entry, u)
	var teams, _ = findTeamMembershipsForUser(eh.DB, u)
	u.TeamMemberships = teams
	var resp = entryGetResponse{
		Status:          "success",
		Body:            entry.Body,
		BodyRendered:    decorateBodyRendered(entry, u, vote),
		GlobalModerator: false,
	}
	enc.Encode(resp)
}

type voteRequest struct {
	VoteType int `json:"vote_type"`
	EntryID  int `json:"entry_id"`
}

func (eh *EntriesHandler) vote(w http.ResponseWriter, req *http.Request) {
	var u, err = getUserFromSession(eh.UserStorage, req)
	var enc = json.NewEncoder(w)
	if err != nil {
		enc.Encode(map[string]string{
			"status":  "error",
			"message": "Error. Logout and try again",
		})
		return
	}

	var payload voteRequest
	var dec = json.NewDecoder(req.Body)
	dec.Decode(&payload)

	var entry dash.Entry
	entry, err = eh.EntryStorage.FindByID(payload.EntryID)
	if err != nil {
		enc.Encode(map[string]string{
			"status":  "error",
			"message": "Error. Logout and try again",
		})
		return
	}

	var vote dash.Vote
	vote, _ = eh.VoteStorage.FindVoteByEntryAndUser(entry, u)
	vote.Type = payload.VoteType
	eh.VoteStorage.Upsert(&vote)
	eh.EntryStorage.UpdateScore(&entry)
}

func (eh *EntriesHandler) delete(w http.ResponseWriter, req *http.Request) {
	var u, err = getUserFromSession(eh.UserStorage, req)
	var enc = json.NewEncoder(w)
	if err != nil {
		enc.Encode(map[string]string{
			"status":  "error",
			"message": "Error. Logout and try again",
		})
		return
	}

	var payload entryGetRequest
	var dec = json.NewDecoder(req.Body)
	dec.Decode(&payload)

	var entry dash.Entry
	entry, err = eh.EntryStorage.FindByID(payload.EntryID)
	if err != nil {
		enc.Encode(map[string]string{
			"status":  "error",
			"message": "Error. Logout and try again",
		})
		return
	}

	if entry.UserID != u.ID {
		enc.Encode(map[string]string{
			"status":  "error",
			"message": "Error. Logout and try again",
		})
		return
	}

	eh.EntryStorage.Delete(&entry)

	enc.Encode(map[string]string{
		"status": "success",
	})
}

func (eh *EntriesHandler) removeFromPublic(w http.ResponseWriter, req *http.Request) {
	var u, err = getUserFromSession(eh.UserStorage, req)
	var enc = json.NewEncoder(w)
	if err != nil || !u.Moderator {
		enc.Encode(map[string]string{
			"status":  "error",
			"message": "Error. Logout and try again",
		})
		return
	}

	var payload entryGetRequest
	var dec = json.NewDecoder(req.Body)
	dec.Decode(&payload)

	var entry dash.Entry
	entry, err = eh.EntryStorage.FindByID(payload.EntryID)
	if err != nil {
		log.Printf("unable to find entry")
		enc.Encode(map[string]string{
			"status":  "error",
			"message": "Error. Logout and try again",
		})
		return
	}

	entry.RemovedFromPublic = true
	if err := eh.EntryStorage.Store(&entry, u); err != nil {
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

func (eh *EntriesHandler) removeFromTeams(w http.ResponseWriter, req *http.Request) {
	var u, err = getUserFromSession(eh.UserStorage, req)
	var enc = json.NewEncoder(w)
	var payload entryGetRequest
	var dec = json.NewDecoder(req.Body)
	dec.Decode(&payload)

	var entry dash.Entry
	entry, err = eh.EntryStorage.FindByID(payload.EntryID)
	if err != nil {
		log.Printf("unable to find entry")
		enc.Encode(map[string]string{
			"status":  "error",
			"message": "Error. Logout and try again",
		})
		return
	}

	var teams, _ = findTeamMembershipsForUser(eh.DB, u)
	u.TeamMemberships = teams

	var isTeamModerator = false
	for _, membership := range u.TeamMemberships {
		for _, team := range entry.Teams {
			isTeamModerator = isTeamModerator ||
				(team == membership.TeamName && (membership.Role == "owner" || membership.Role == "moderator"))
		}
	}
	if err != nil || !isTeamModerator {
		log.Printf("You are no team moderator")
		enc.Encode(map[string]string{
			"status":  "error",
			"message": "Error. Logout and try again",
		})
		return
	}

	if err := eh.EntryStorage.RemoveFromTeams(entry, u); err != nil {
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

func (eh *EntriesHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	switch req.URL.Path {
	case "list":
		eh.list(w, req)
	case "save":
		eh.save(w, req)
	case "create":
		eh.save(w, req)
	case "get":
		eh.get(w, req)
	case "vote":
		eh.vote(w, req)
	case "delete":
		eh.delete(w, req)
	case "remove_from_public":
		eh.removeFromPublic(w, req)
	case "remove_from_teams":
		eh.removeFromTeams(w, req)
	default:
		log.Printf("Unknown entries route: %v", req.URL.Path)
	}
}
