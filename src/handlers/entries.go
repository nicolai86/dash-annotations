package handlers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"text/template"
	"time"

	"github.com/russross/blackfriday"

	"golang.org/x/net/context"

	"dash"
)

var (
	ErrMissingTitle              = errors.New("Missing parameter: title")
	ErrMissingBody               = errors.New("Missing parameter: body")
	ErrMissingAnchor             = errors.New("Missing parameter: anchor")
	ErrPublicAnnotationForbidden = errors.New("Public annotations forbidden")
	ErrUpdateForbidden           = errors.New("You need to be the author")
	ErrDeleteForbidden           = errors.New("Only the author can delete an entry")
	ErrNotModerator              = errors.New("You need to be a moderator for this")
	ErrNotTeamModerator          = errors.New("You need to be the teams moderator for this")
)

func findVoteByEntryAndUser(db *sql.DB, entry dash.Entry, u dash.User) (dash.Vote, error) {
	var vote = dash.Vote{
		EntryID: entry.ID,
		UserID:  u.ID,
	}
	var err = db.QueryRow(`SELECT id, type, entry_id, user_id FROM votes WHERE entry_id = ? AND user_id = ?`, entry.ID, u.ID).Scan(&vote.ID, &vote.Type, &vote.EntryID, &vote.UserID)
	return vote, err
}

func findTeamMembershipsForUser(db *sql.DB, u dash.User) ([]dash.TeamMember, error) {
	var rows, err = db.Query(`SELECT t.id, t.name, tm.role FROM team_user AS tm INNER JOIN teams AS t ON t.id = tm.team_id WHERE tm.user_id = ?`, u.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

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

func FindByTeamAndIdentifier(db *sql.DB, identifier dash.IdentifierDict, user dash.User) ([]dash.Entry, error) {
	if len(user.TeamMemberships) < 1 {
		return nil, nil
	}

	if err := upsertIdentifier(db, &identifier); err != nil {
		return nil, err
	}

	var query = fmt.Sprintf(`SELECT e.id, e.title, e.type, e.anchor, e.body, e.body_rendered, e.score, e.user_id
		FROM entries e
		INNER JOIN entry_team et ON et.entry_id = e.id
		WHERE identifier_id = ?
			AND et.removed_from_team = ?
			AND e.user_id != ?
			AND et.team_id IN (%s)
		GROUP BY e.id`, strings.Join(strings.Split(strings.Repeat("?", len(user.TeamMemberships)), ""), ","))
	var params = []interface{}{identifier.ID, false, user.ID}
	for _, membership := range user.TeamMemberships {
		params = append(params, membership.TeamID)
	}
	var rows, err = db.Query(query, params...)
	defer rows.Close()
	if err != nil {
		return nil, err
	}

	var entries = make([]dash.Entry, 0)
	for rows.Next() {
		var entry = dash.Entry{}
		if err := rows.Scan(&entry.ID, &entry.Title, &entry.Type, &entry.Anchor, &entry.Body, &entry.BodyRendered, &entry.Score, &entry.UserID); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

func FindPublicByIdentifier(db *sql.DB, identifier dash.IdentifierDict, user *dash.User) ([]dash.Entry, error) {
	if err := upsertIdentifier(db, &identifier); err != nil {
		return nil, err
	}

	var query = `SELECT
    e.id,
    e.title,
    e.type,
    e.anchor,
    e.body,
    e.body_rendered,
    e.score,
    e.user_id
  FROM entries e
    WHERE e.identifier_id = ?
    AND e.public = ?
    AND e.removed_from_public = ?
    AND e.score > ?
	`
	var params = []interface{}{identifier.ID, true, false, -5}
	if user != nil && len(user.TeamMemberships) > 0 {
		var subQuery = fmt.Sprintf(`SELECT e.id
      FROM entries e
      INNER JOIN entry_team et ON et.entry_id = e.id
      WHERE identifier_id = ?
        AND et.removed_from_team = ?
        AND et.team_id IN (%s)
      GROUP BY e.id`, strings.Join(strings.Split(strings.Repeat("?", len(user.TeamMemberships)), ""), ","))

		query = query + "AND e.id NOT IN (" + subQuery + ")"
		params = append(params, identifier.ID)
		params = append(params, true)
		for _, team := range user.TeamMemberships {
			params = append(params, team.TeamID)
		}
	}
	if user != nil {
		query += ` AND user_id != ?`
		params = append(params, user.ID)
	}
	var rows, err = db.Query(query, params...)
	defer rows.Close()
	if err != nil {
		return nil, err
	}

	var entries = make([]dash.Entry, 0)
	for rows.Next() {
		var entry = dash.Entry{}
		if err := rows.Scan(&entry.ID, &entry.Title, &entry.Type, &entry.Anchor, &entry.Body, &entry.BodyRendered, &entry.Score, &entry.UserID); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

func FindOwnByIdentifier(db *sql.DB, identifier dash.IdentifierDict, user *dash.User) ([]dash.Entry, error) {
	if err := upsertIdentifier(db, &identifier); err != nil {
		return nil, err
	}

	if user == nil {
		return nil, nil
	}

	var rows, err = db.Query(`SELECT id, title, type, anchor, body, body_rendered, score, user_id FROM entries WHERE user_id = ?`, user.ID)
	defer rows.Close()
	if err != nil {
		return nil, err
	}

	var entries = make([]dash.Entry, 0)
	for rows.Next() {
		var entry = dash.Entry{}
		if err := rows.Scan(&entry.ID, &entry.Title, &entry.Type, &entry.Anchor, &entry.Body, &entry.BodyRendered, &entry.Score, &entry.UserID); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

type listRequest struct {
	Identifier dash.IdentifierDict `json:"identifier"`
}

type listResponse struct {
	Status        string       `json:"status"`
	PublicEntries []dash.Entry `json:"public_entries,omitempty"`
	OwnEntries    []dash.Entry `json:"own_entries,omitempty"`
	TeamEntries   []dash.Entry `json:"team_entries,omitempty"`
}

func EntriesList(ctx context.Context, w http.ResponseWriter, req *http.Request) error {
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
	var listReq listRequest
	dec.Decode(&listReq)
	var enc = json.NewEncoder(w)

	var public, own, team []dash.Entry
	var err error
	if public, err = FindPublicByIdentifier(db, listReq.Identifier, user); err != nil {
		return err
	}
	if own, err = FindOwnByIdentifier(db, listReq.Identifier, user); err != nil {
		return err
	}
	if user != nil {
		var err error
		if team, err = FindByTeamAndIdentifier(db, listReq.Identifier, *user); err != nil {
			return err
		}
	}
	// TODO remove from public which are in team

	var resp = listResponse{
		Status:        "success",
		PublicEntries: public,
		OwnEntries:    own,
		TeamEntries:   team,
	}
	enc.Encode(resp)
	return nil
}

type eSaveRequest struct {
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

type eSaveResponse struct {
	Status string     `json:"status"`
	Entry  dash.Entry `json:"entry"`
}

func upsertIdentifier(db *sql.DB, dict *dash.IdentifierDict) error {
	if dict.DocsetFilename == "Mono" && dict.HttrackSource != "" {
		db.QueryRow(`SELECT id FROM identifiers WHERE docset_filename = ? AND httrack_source = ? LIMIT 1`, dict.DocsetFilename, dict.HttrackSource).Scan(&dict.ID)
	} else {
		db.QueryRow(`SELECT id FROM identifiers WHERE docset_filename = ? AND page_path = ? LIMIT 1`, dict.DocsetFilename, dict.PagePath).Scan(&dict.ID)
	}

	if dict.ID == 0 {
		var res, err = db.Exec(`INSERT INTO identifiers
      (docset_name, docset_filename, docset_platform, docset_bundle, docset_version, page_path, page_title, httrack_source, banned_from_public, created_at)
      VALUES
      (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, dict.DocsetName, dict.DocsetFilename, dict.DocsetPlatform, dict.DocsetBundle, dict.DocsetVersion, dict.PagePath, dict.PageTitle, dict.HttrackSource, 0, time.Now())
		if err != nil {
			return err
		}
		var dictID int64
		dictID, err = res.LastInsertId()
		dict.ID = int(dictID)
		return err
	}
	return nil
}

func upsertEntry(db *sql.DB, entry *dash.Entry, author *dash.User) error {
	if entry.Title == "" {
		return ErrMissingTitle
	}
	if entry.Body == "" {
		return ErrMissingBody
	}
	if entry.Anchor == "" {
		return ErrMissingAnchor
	}
	if err := upsertIdentifier(db, &entry.Identifier); err != nil {
		return err
	}
	if entry.Public && entry.Identifier.BannedFromPublic {
		return ErrPublicAnnotationForbidden
	}
	var createVote = false
	if entry.ID != 0 && !author.Moderator {
		if err := db.QueryRow(`SELECT user_id FROM entries WHERE id = ?`, entry.ID).Scan(&entry.UserID); err != nil {
			return err
		}
		if entry.UserID != author.ID {
			return ErrUpdateForbidden
		}
	} else {
		entry.Score = 1
		createVote = true
	}
	entry.IdentifierID = entry.Identifier.ID
	entry.BodyRendered = string(blackfriday.MarkdownCommon([]byte(entry.Body)))

	if entry.ID != 0 {
		var _, err = db.Exec(`UPDATE entries SET
				title               = ?,
				body                = ?,
				body_rendered       = ?,
				type                = ?,
				identifier_id       = ?,
				anchor              = ?,
				public              = ?,
				removed_from_public = ?,
				score               = ?,
				updated_at          = ?
			WHERE id = ?`,
			entry.Title,
			entry.Body,
			entry.BodyRendered,
			entry.Type,
			entry.IdentifierID,
			entry.Anchor,
			entry.Public,
			entry.RemovedFromPublic,
			entry.Score,
			time.Now(), entry.ID)
		if err != nil {
			return err
		}
	} else {
		var res, err = db.Exec(`INSERT INTO entries (title, body, body_rendered, type, identifier_id, anchor, public, removed_from_public, score, user_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			entry.Title, entry.Body, entry.BodyRendered, entry.Type, entry.IdentifierID, entry.Anchor, entry.Public, entry.RemovedFromPublic, entry.Score, author.ID, time.Now(), time.Now())
		if err != nil {
			return err
		}
		var insertID int64
		insertID, err = res.LastInsertId()
		if err != nil {
			return err
		}
		entry.ID = int(insertID)
	}

	if createVote {
		// TODO initial vote
		//         $vote = new Vote;
		//         $vote->type = 1;
		//         $vote->user_id = $user->id;
		//         $vote->entry_id = $entry->id;
		//         $vote->save();
	}

	for _, t := range entry.Teams {
		var teamID int64
		db.QueryRow(`SELECT id FROM teams WHERE name = ? LIMIT 1`, t).Scan(&teamID)
		db.Exec(`INSERT INTO entry_team (entry_id, team_id) VALUES (?, ?)`, entry.ID, teamID)
	}

	return nil
}

func EntriesSave(ctx context.Context, w http.ResponseWriter, req *http.Request) error {
	var db = ctx.Value(DBKey).(*sql.DB)
	var user = ctx.Value(UserKey).(*dash.User)

	var payload eSaveRequest
	json.NewDecoder(req.Body).Decode(&payload)

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
	if err := upsertEntry(db, &entry, user); err != nil {
		return err
	}

	var vote = dash.Vote{
		EntryID: entry.ID,
		UserID:  user.ID,
		Type:    dash.VoteUp,
	}
	if _, err := db.Exec(`INSERT INTO votes (type, entry_id, user_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`, vote.Type, vote.EntryID, vote.UserID, time.Now(), time.Now()); err != nil {
		return err
	}
	updateEntryVoteScore(db, &entry)

	json.NewEncoder(w).Encode(eSaveResponse{
		Entry:  entry,
		Status: "success",
	})
	return nil
}

type eGetRequest struct {
	EntryID int `json:"entry_id"`
}
type eGetResponse struct {
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
	html, err = template.New("get.html").Funcs(fns).ParseFiles("./templates/entries/get.html", "./templates/entries/uncss.css")
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

func EntryGet(ctx context.Context, w http.ResponseWriter, req *http.Request) error {
	var db = ctx.Value(DBKey).(*sql.DB)
	var entry = ctx.Value(EntryKey).(*dash.Entry)

	var user = dash.User{}
	if ctx.Value(UserKey) != nil {
		user = dash.User(*ctx.Value(UserKey).(*dash.User))
	}

	var vote, _ = findVoteByEntryAndUser(db, *entry, user)
	var err error
	var teams []dash.TeamMember
	if teams, err = findTeamMembershipsForUser(db, user); err != nil {
		return err
	}
	user.TeamMemberships = teams
	var resp = eGetResponse{
		Status:          "success",
		Body:            entry.Body,
		BodyRendered:    decorateBodyRendered(*entry, user, vote),
		GlobalModerator: false,
	}
	json.NewEncoder(w).Encode(resp)
	return nil
}

type voteRequest struct {
	VoteType int `json:"vote_type"`
	EntryID  int `json:"entry_id"`
}

func updateEntryVoteScore(db *sql.DB, entry *dash.Entry) error {
	var score = 0
	var err = db.QueryRow(`SELECT SUM(type) FROM votes WHERE entry_id = ?`, entry.ID).Scan(&score)
	if err != nil {
		return err
	}

	_, err = db.Exec(`UPDATE entries SET score = ? WHERE id = ?`, score, entry.ID)
	entry.Score = score
	return err
}

func EntryVote(ctx context.Context, w http.ResponseWriter, req *http.Request) error {
	var db = ctx.Value(DBKey).(*sql.DB)
	var user = ctx.Value(UserKey).(*dash.User)
	var entry = ctx.Value(EntryKey).(*dash.Entry)

	var payload voteRequest
	json.NewDecoder(req.Body).Decode(&payload)

	var vote dash.Vote
	var err error
	if vote, err = findVoteByEntryAndUser(db, *entry, *user); err != nil {
		return err
	}
	vote.Type = payload.VoteType

	if vote.ID != 0 {
		_, err = db.Exec(`UPDATE votes SET type = ?, updated_at = ? WHERE entry_id = ? AND user_id = ?`, vote.Type, time.Now(), vote.EntryID, vote.UserID)
	} else {
		_, err = db.Exec(`INSERT INTO votes (type, entry_id, user_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`, vote.Type, vote.EntryID, vote.UserID, time.Now(), time.Now())
	}
	if err != nil {
		return err
	}

	updateEntryVoteScore(db, entry)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "success",
	})
	return nil
}

func EntryDelete(ctx context.Context, w http.ResponseWriter, req *http.Request) error {
	var db = ctx.Value(DBKey).(*sql.DB)
	var user = ctx.Value(UserKey).(*dash.User)
	var entry = ctx.Value(EntryKey).(*dash.Entry)

	var payload eGetRequest
	json.NewDecoder(req.Body).Decode(&payload)

	if entry.UserID != user.ID {
		return ErrDeleteForbidden
	}

	if _, err := db.Exec(`DELETE FROM votes WHERE entry_id = ?`, entry.ID); err != nil {
		return err
	}

	if _, err := db.Exec(`DELETE FROM entry_team WHERE entry_id = ?`, entry.ID); err != nil {
		return err
	}

	if _, err := db.Exec(`DELETE FROM entries WHERE id = ?`, entry.ID); err != nil {
		return err
	}

	json.NewEncoder(w).Encode(map[string]string{
		"status": "success",
	})
	return nil
}

func EntryRemoveFromPublic(ctx context.Context, w http.ResponseWriter, req *http.Request) error {
	var user = ctx.Value(UserKey).(*dash.User)
	if !user.Moderator {
		return ErrNotModerator
	}

	var db = ctx.Value(DBKey).(*sql.DB)
	var entry = ctx.Value(EntryKey).(*dash.Entry)

	if _, err := db.Exec(`UPDATE entries SET removed_from_public = ? WHERE id = ?`, true, entry.ID); err != nil {
		return err
	}

	json.NewEncoder(w).Encode(map[string]string{
		"status": "success",
	})
	return nil
}

func EntryRemoveFromTeams(ctx context.Context, w http.ResponseWriter, req *http.Request) error {
	var db = ctx.Value(DBKey).(*sql.DB)
	var user = ctx.Value(UserKey).(*dash.User)
	var entry = ctx.Value(EntryKey).(*dash.Entry)

	var payload eGetRequest
	json.NewDecoder(req.Body).Decode(&payload)

	var teams, _ = findTeamMembershipsForUser(db, *user)
	user.TeamMemberships = teams

	var isTeamModerator = false
	for _, membership := range user.TeamMemberships {
		for _, team := range entry.Teams {
			isTeamModerator = isTeamModerator ||
				(team == membership.TeamName && (membership.Role == "owner" || membership.Role == "moderator"))
		}
	}
	if !isTeamModerator {
		return ErrNotTeamModerator
	}

	var args = []interface{}{
		true,
		entry.ID,
	}
	for _, membership := range user.TeamMemberships {
		args = append(args, membership.TeamID)
	}
	query := fmt.Sprintf(`UPDATE entry_team SET removed_from_team = ? WHERE entry_id = ? AND team_id IN (%s)`,
		strings.Join(strings.Split(strings.Repeat("?", len(user.TeamMemberships)), ""), ","))

	if _, err := db.Exec(query, args...); err != nil {
		return err
	}

	json.NewEncoder(w).Encode(map[string]string{
		"status": "success",
	})
	return nil
}
