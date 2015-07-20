package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/context"

	"dash"
)

var (
	// ErrInvalidAccessKey is returned from a join request handler when the given access key
	// does not match the teams access key
	ErrInvalidAccessKey = errors.New("Invalid access key")
	// ErrMissingRoleParameter is returned from a set_role handler when no role parameter is present
	ErrMissingRoleParameter = errors.New("Missing parameter: role")
	// ErrInvalidRoleParameter is returned from the set_role handler when the role is unknown
	ErrInvalidRoleParameter = errors.New("Invalid parameter: role. Must either be member or moderator")
	// ErrMissingUsernameParameter is returned when a target username parameter is required, but missing
	ErrMissingUsernameParameter = errors.New("Missing parameter: username")
	// ErrNotTeamOwner is returned when an action requires you to be the team owner, but you are not
	ErrNotTeamOwner = errors.New("You need to be the teams owner")
	// ErrUnknownUser is returned when an action requires a target user, but the given username is unknown
	ErrUnknownUser = errors.New("Invalid parameter: username. Unknown user")
	// ErrTeamNameExists is returned when a team should be created, and the name is already taken
	ErrTeamNameExists = errors.New("A team with this name already exists")
	// ErrTeamNameMissing is returned when a team should be created and the name parameter is missing
	ErrTeamNameMissing = errors.New("Missing parameter: name")
)

type teamListResponse struct {
	Status string            `json:"status"`
	Teams  []dash.TeamMember `json:"teams,omitempty"`
}

// TeamList returns a list of all teams the current user is a member|moderator of
func TeamList(ctx context.Context, w http.ResponseWriter, req *http.Request) error {
	var user = ctx.Value(UserKey).(*dash.User)

	json.NewEncoder(w).Encode(teamListResponse{
		Status: "success",
		Teams:  user.TeamMemberships,
	})
	return nil
}

// TeamCreate tries to create a new team with a given name inside the database
func TeamCreate(ctx context.Context, w http.ResponseWriter, req *http.Request) error {
	var db = ctx.Value(DBKey).(*sql.DB)
	var user = ctx.Value(UserKey).(*dash.User)

	var payload map[string]interface{}
	json.NewDecoder(req.Body).Decode(&payload)

	var team = dash.Team{
		Name: payload["name"].(string),
	}
	if team.Name == "" {
		return ErrTeamNameMissing
	}

	var tx, err = db.Begin()
	if err != nil {
		return err
	}

	var cnt = 0
	tx.QueryRow(`SELECT count(*) FROM teams WHERE name = ?`, team.Name).Scan(&cnt)

	if cnt != 0 {
		return ErrTeamNameExists
	}

	var res, _ = tx.Exec(`INSERT INTO teams (name, access_key, created_at, updated_at) VALUES (?, ?, ?, ?)`, team.Name, "", time.Now(), time.Now())

	var teamID int64
	teamID, err = res.LastInsertId()
	if err != nil {
		return err
	}
	team.ID = int(teamID)

	tx.Exec(`INSERT INTO team_user (team_id, user_id, role) VALUES (?, ?, ?)`, team.ID, user.ID, "owner")
	if err := tx.Commit(); err != nil {
		return err
	}

	json.NewEncoder(w).Encode(map[string]string{
		"status": "success",
	})
	return nil
}

type teamJoinRequest struct {
	AccessKey string `json:"access_key"`
}

// TeamJoin tries to add the current user to the requested team
func TeamJoin(ctx context.Context, w http.ResponseWriter, req *http.Request) error {
	var db = ctx.Value(DBKey).(*sql.DB)
	var user = ctx.Value(UserKey).(*dash.User)
	var targetTeam = ctx.Value(TeamKey).(*dash.Team)

	var dec = json.NewDecoder(req.Body)

	var payload teamJoinRequest
	dec.Decode(&payload)

	if !targetTeam.AccessKeysMatch(payload.AccessKey) {
		return errors.New("Invalid access key")
	}

	if _, err := db.Exec(`INSERT INTO team_user (team_id, user_id, role) VALUES (?, ?, ?)`, targetTeam.ID, user.ID, "member"); err != nil {
		return err
	}

	json.NewEncoder(w).Encode(map[string]string{
		"status": "success",
	})
	return nil
}

// TeamLeave removes the current user from the requested team
func TeamLeave(ctx context.Context, w http.ResponseWriter, req *http.Request) error {
	var db = ctx.Value(DBKey).(*sql.DB)
	var user = ctx.Value(UserKey).(*dash.User)
	var team = ctx.Value(TeamKey).(*dash.Team)

	var enc = json.NewEncoder(w)

	var tx, err = db.Begin()
	if err != nil {
		return err
	}

	tx.Exec(`DELETE FROM team_user WHERE team_id = ? AND user_id = ?`, team.ID, user.ID)

	var entryIDs = make([]interface{}, 0)
	var rows *sql.Rows
	rows, err = tx.Query(`SELECT e.id FROM entries e INNER JOIN entry_team et ON et.entry_id = e.id AND et.team_id = ? WHERE e.user_id = ?`, team.ID, user.ID)
	defer rows.Close()
	for rows.Next() {
		var entryID int
		if err := rows.Scan(&entryID); err != nil {
			return err
		}
		entryIDs = append(entryIDs, entryID)
	}

	params := append([]interface{}{user.ID}, entryIDs...)
	var query = fmt.Sprintf(`DELETE FROM votes WHERE user_id = ? AND entry_id IN (%s) `, strings.Join(strings.Split(strings.Repeat("?", len(entryIDs)), ""), ","))
	tx.Exec(query, params...)

	query = fmt.Sprintf(`DELETE FROM entry_team WHERE entry_id IN (%s) `, strings.Join(strings.Split(strings.Repeat("?", len(entryIDs)), ""), ","))
	tx.Exec(query, entryIDs...)

	query = fmt.Sprintf(`DELETE FROM entries WHERE id IN (%s) `, strings.Join(strings.Split(strings.Repeat("?", len(entryIDs)), ""), ","))
	tx.Exec(query, entryIDs...)

	var membershipCount = -1
	tx.QueryRow(`SELECT count(*) from team_user WHERE team_id = ?`, team.ID).Scan(&membershipCount)
	if membershipCount == 0 {
		tx.Exec(`DELETE FROM teams WHERE id = ?`, team.ID)
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	enc.Encode(map[string]string{
		"status": "success",
	})
	return nil
}

type teamSetRoleRequest struct {
	Role     string `json:"role"`
	Username string `json:"username"`
}

// TeamSetRole changes the role of the target user to the requested role
func TeamSetRole(ctx context.Context, w http.ResponseWriter, req *http.Request) error {
	var db = ctx.Value(DBKey).(*sql.DB)
	var user = ctx.Value(UserKey).(*dash.User)
	var team = ctx.Value(TeamKey).(*dash.Team)

	if team.OwnerID != user.ID {
		return ErrNotTeamOwner
	}

	var dec = json.NewDecoder(req.Body)

	var payload teamSetRoleRequest
	dec.Decode(&payload)

	if payload.Role == "" {
		return ErrMissingRoleParameter
	}
	if payload.Role != "member" && payload.Role != "moderator" {
		return ErrInvalidRoleParameter
	}
	if payload.Username == "" {
		return ErrMissingUsernameParameter
	}
	var target, err = findUserByUsername(db, payload.Username)
	if err != nil {
		return ErrUnknownUser
	}

	if _, err := db.Exec(`UPDATE team_user SET role = ? WHERE team_id = ? AND user_id = ?`, payload.Role, team.ID, target.ID); err != nil {
		return err
	}

	json.NewEncoder(w).Encode(map[string]string{
		"status": "success",
	})
	return nil
}

type teamRemoveMemberRequest struct {
	Username string `json:"username"`
}

// TeamRemoveMember allows a moderator to remove a member from a team
func TeamRemoveMember(ctx context.Context, w http.ResponseWriter, req *http.Request) error {
	var db = ctx.Value(DBKey).(*sql.DB)
	var user = ctx.Value(UserKey).(*dash.User)
	var team = ctx.Value(TeamKey).(*dash.Team)

	if team.OwnerID != user.ID {
		return ErrNotTeamOwner
	}

	var payload teamRemoveMemberRequest
	json.NewDecoder(req.Body).Decode(&payload)

	if payload.Username == "" {
		return ErrMissingUsernameParameter
	}

	var target, err = findUserByUsername(db, payload.Username)
	if err != nil {
		return ErrUnknownUser
	}

	var tx *sql.Tx
	if tx, err = db.Begin(); err != nil {
		return err
	}

	tx.Exec(`DELETE FROM team_user WHERE team_id = ? AND user_id = ?`, team.ID, target.ID)

	var entryIDs = make([]interface{}, 0)
	var rows *sql.Rows
	rows, err = tx.Query(`SELECT e.id FROM entries e INNER JOIN entry_team et ON et.entry_id = e.id AND et.team_id = ? WHERE e.user_id = ?`, team.ID, target.ID)
	defer rows.Close()
	if err != nil {
		return err
	}
	for rows.Next() {
		var entryID int
		if err := rows.Scan(&entryID); err != nil {
			return err
		}
		entryIDs = append(entryIDs, entryID)
	}

	params := append([]interface{}{target.ID}, entryIDs...)
	var query = fmt.Sprintf(`DELETE FROM votes WHERE user_id = ? AND entry_id IN (%s) `, strings.Join(strings.Split(strings.Repeat("?", len(entryIDs)), ""), ","))
	tx.Exec(query, params...)

	query = fmt.Sprintf(`DELETE FROM entry_team WHERE entry_id IN (%s) `, strings.Join(strings.Split(strings.Repeat("?", len(entryIDs)), ""), ","))
	tx.Exec(query, entryIDs...)

	query = fmt.Sprintf(`DELETE FROM entries WHERE id IN (%s) `, strings.Join(strings.Split(strings.Repeat("?", len(entryIDs)), ""), ","))
	tx.Exec(query, entryIDs...)

	if err := tx.Commit(); err != nil {
		return err
	}

	json.NewEncoder(w).Encode(map[string]string{
		"status": "success",
	})
	return nil
}

type teamSetAccessKeyRequest struct {
	AccessKey string `json:"access_key"`
}

// TeamSetAccessKey allows moderators to change the access key for a given team
func TeamSetAccessKey(ctx context.Context, w http.ResponseWriter, req *http.Request) error {
	var db = ctx.Value(DBKey).(*sql.DB)
	var user = ctx.Value(UserKey).(*dash.User)
	var team = ctx.Value(TeamKey).(*dash.Team)

	if team.OwnerID != user.ID {
		return ErrNotTeamOwner
	}

	var enc = json.NewEncoder(w)

	var payload teamSetAccessKeyRequest
	json.NewDecoder(req.Body).Decode(&payload)

	team.ChangeAccessKey(payload.AccessKey)

	if _, err := db.Exec(`UPDATE teams SET access_key = ? WHERE id = ?`, team.EncryptedAccessKey, team.ID); err != nil {
		return err
	}

	enc.Encode(map[string]string{
		"status": "success",
	})
	return nil
}

type membership struct {
	Role     string `json:"role"`
	Username string `json:"name"`
}

type teamListMembersResponse struct {
	Status       string       `json:"status"`
	Members      []membership `json:"members"`
	HasAccessKey bool         `json:"has_access_key"`
}

// TeamListMember allows moderators to list all members of a requested team
func TeamListMember(ctx context.Context, w http.ResponseWriter, req *http.Request) error {
	var db = ctx.Value(DBKey).(*sql.DB)
	var user = ctx.Value(UserKey).(*dash.User)
	var team = ctx.Value(TeamKey).(*dash.Team)

	if team.OwnerID != user.ID {
		return ErrNotTeamOwner
	}

	var payload map[string]interface{}
	json.NewDecoder(req.Body).Decode(&payload)

	var rows, err = db.Query(`SELECT tm.role, u.username FROM team_user AS tm INNER JOIN users AS u ON u.id = tm.user_id WHERE tm.team_id = ?`, team.ID)
	if err != nil {
		return err
	}
	defer rows.Close()

	var memberships = make([]membership, 0)
	for rows.Next() {
		var membership = membership{}
		if err := rows.Scan(&membership.Role, &membership.Username); err != nil {
			return err
		}
		memberships = append(memberships, membership)
	}

	var resp = teamListMembersResponse{
		Status:       "success",
		Members:      memberships,
		HasAccessKey: team.EncryptedAccessKey != "",
	}
	json.NewEncoder(w).Encode(resp)
	return nil
}
