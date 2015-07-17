package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/context"

	"dash"
)

type teamListResponse struct {
	Status string            `json:"status"`
	Teams  []dash.TeamMember `json:"teams,omitempty"`
}

const teamListQuery = `
	SELECT
		t.id,
		t.name,
		tm.role
	FROM team_user AS tm
	INNER JOIN teams AS t ON t.id = tm.team_id
	WHERE tm.user_id = ?`

func TeamsList(ctx context.Context, w http.ResponseWriter, req *http.Request) {
	var db = ctx.Value(DBKey).(*sql.DB)
	var user = ctx.Value(UserKey).(*dash.User)

	var enc = json.NewEncoder(w)

	var rows, err = db.Query(teamListQuery, user.ID)
	defer rows.Close()
	if err != nil {
		// TODO
		return
	}

	var memberships = make([]dash.TeamMember, 0)
	for rows.Next() {
		var membership = dash.TeamMember{}
		if err := rows.Scan(&membership.TeamID, &membership.TeamName, &membership.Role); err != nil {
			// TODO
			return
		}
		memberships = append(memberships, membership)
	}

	var resp = teamListResponse{
		Status: "success",
		Teams:  memberships,
	}
	enc.Encode(resp)
}

func TeamCreate(ctx context.Context, w http.ResponseWriter, req *http.Request) {
	var db = ctx.Value(DBKey).(*sql.DB)
	var user = ctx.Value(UserKey).(*dash.User)

	var enc = json.NewEncoder(w)
	var dec = json.NewDecoder(req.Body)

	var payload map[string]interface{}
	dec.Decode(&payload)

	var team = dash.Team{
		Name: payload["name"].(string),
	}

	var err = (func() error {
		if team.Name == "" {
			return ErrNameMissing
		}

		var isNewTeam = team.ID == 0
		if isNewTeam {
			var cnt = 0
			var err = db.QueryRow(`SELECT count(*) FROM teams WHERE name = ?`, team.Name).Scan(&cnt)
			if err != nil {
				return err
			}
			if cnt != 0 {
				return ErrNameExists
			}
			var ins, insErr = db.Exec(`INSERT INTO teams (name, created_at, updated_at) VALUES (?, ?, ?)`, team.Name, time.Now(), time.Now())
			var teamID int64
			teamID, _ = ins.LastInsertId()
			team.ID = int(teamID)

			return insErr
		}

		db.Exec(`UPDATE teams SET access_key = ? WHERE id = ?`, team.EncryptedAccessKey, team.ID)
		return nil
	})()

	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		switch err {
		case ErrNameExists:
			enc.Encode(map[string]string{
				"status":  "error",
				"message": "Team name already taken",
			})
		case ErrNameMissing:
			enc.Encode(map[string]string{
				"status":  "error",
				"message": "Missing name property",
			})
		default:
			enc.Encode(map[string]string{
				"status": "error",
			})
		}
		return
	}

	db.Exec(`INSERT INTO team_user (team_id, user_id, role) VALUES (?, ?, ?)`, team.ID, user.ID, "owner")

	enc.Encode(map[string]string{
		"status": "success",
	})
}

func TeamJoin(ctx context.Context, w http.ResponseWriter, req *http.Request) {
	var db = ctx.Value(DBKey).(*sql.DB)
	var user = ctx.Value(UserKey).(*dash.User)
	var team = ctx.Value(TeamKey).(*dash.Team)

	var enc = json.NewEncoder(w)
	var dec = json.NewDecoder(req.Body)

	var payload map[string]interface{}
	dec.Decode(&payload)
	log.Printf("%#v", payload)

	if !team.AccessKeysMatch(payload["access_key"].(string)) {
		enc.Encode(map[string]string{
			"status":  "error",
			"message": "Invalid access key",
		})
		return
	}

	if _, err := db.Exec(`INSERT INTO team_user (team_id, user_id, role) VALUES (?, ?, ?)`, team.ID, user.ID, "member"); err != nil {
		enc.Encode(map[string]string{
			"status":  "error",
			"message": "You are already a member of this team"})
		return
	}

	enc.Encode(map[string]string{
		"status": "success",
	})
}

func TeamLeave(ctx context.Context, w http.ResponseWriter, req *http.Request) {
	var db = ctx.Value(DBKey).(*sql.DB)
	var user = ctx.Value(UserKey).(*dash.User)
	var team = ctx.Value(TeamKey).(*dash.Team)

	var enc = json.NewEncoder(w)

	var _, err = db.Exec(`DELETE FROM team_user WHERE team_id = ? AND user_id = ?`, team.ID, user.ID)
	if err != nil {
	}

	var entryIDs = make([]interface{}, 0)
	var rows *sql.Rows
	rows, err = db.Query(`SELECT e.id FROM entries e INNER JOIN entry_team et ON et.entry_id = e.id AND et.team_id = ? WHERE e.user_id = ?`, team.ID, user.ID)
	defer rows.Close()
	for rows.Next() {
		var entryID int
		if err := rows.Scan(&entryID); err != nil {
			// TODO
			return
		}
		entryIDs = append(entryIDs, entryID)
	}

	params := append([]interface{}{user.ID}, entryIDs...)
	var query = fmt.Sprintf(`DELETE FROM votes WHERE user_id = ? AND entry_id IN (%s) `, strings.Join(strings.Split(strings.Repeat("?", len(entryIDs)), ""), ","))
	db.Exec(query, params...)

	query = fmt.Sprintf(`DELETE FROM entry_team WHERE entry_id IN (%s) `, strings.Join(strings.Split(strings.Repeat("?", len(entryIDs)), ""), ","))
	db.Exec(query, entryIDs...)

	query = fmt.Sprintf(`DELETE FROM entries WHERE id IN (%s) `, strings.Join(strings.Split(strings.Repeat("?", len(entryIDs)), ""), ","))
	db.Exec(query, entryIDs...)

	var membershipCount = -1
	db.QueryRow(`SELECT count(*) from team_user WHERE team_id = ?`, team.ID).Scan(&membershipCount)
	if membershipCount == 0 {
		db.Exec(`DELETE FROM teams WHERE id = ?`, team.ID)
	}

	enc.Encode(map[string]string{
		"status": "success",
	})
}

func TeamSetRole(ctx context.Context, w http.ResponseWriter, req *http.Request) {
	var db = ctx.Value(DBKey).(*sql.DB)
	var user = ctx.Value(UserKey).(*dash.User)

	var enc = json.NewEncoder(w)
	var dec = json.NewDecoder(req.Body)

	var payload map[string]interface{}
	dec.Decode(&payload)

	var team = ctx.Value(TeamKey).(*dash.Team)

	var role = payload["role"].(string)
	var username = payload["username"].(string)

	if role == "" || username == "" {
		enc.Encode(map[string]string{
			"status":  "error",
			"message": "Missing Role | Username",
		})
		return
	}

	if team.OwnerID != user.ID {
		enc.Encode(map[string]string{
			"status":  "error",
			"message": "Error. Need to be owner.",
		})
	}

	var target dash.User
	var err error
	target, err = FindUserByUsername(db, username)
	if err != nil {
		enc.Encode(map[string]string{
			"status":  "error",
			"message": "Unknown user",
		})
		return
	}

	db.Exec(`UPDATE team_user SET role = ? WHERE team_id = ? AND user_id = ?`, role, team.ID, target.ID)

	enc.Encode(map[string]string{
		"status": "success",
	})
}

func TeamRemoveMember(ctx context.Context, w http.ResponseWriter, req *http.Request) {
	var db = ctx.Value(DBKey).(*sql.DB)
	var user = ctx.Value(UserKey).(*dash.User)
	var team = ctx.Value(TeamKey).(*dash.Team)

	var enc = json.NewEncoder(w)
	var dec = json.NewDecoder(req.Body)

	var payload map[string]interface{}
	dec.Decode(&payload)

	var username = payload["username"].(string)
	if username == "" {
		enc.Encode(map[string]string{
			"status":  "error",
			"message": "Missing Role | Username",
		})
		return
	}

	if team.OwnerID != user.ID {
		enc.Encode(map[string]string{
			"status":  "error",
			"message": "Error. Need to be owner.",
		})
	}

	var target dash.User
	var err error
	target, err = FindUserByUsername(db, username)
	if err != nil {
		enc.Encode(map[string]string{
			"status":  "error",
			"message": "Unknown user",
		})
		return
	}

	if _, err := db.Exec(`DELETE FROM team_user WHERE team_id = ? AND user_id = ?`, team.ID, target.ID); err != nil {
		log.Printf("%v", err)
		enc.Encode(map[string]string{
			"status":  "error",
			"message": "Unknown user",
		})
		return
	}

	var entryIDs = make([]interface{}, 0)
	var rows *sql.Rows
	rows, err = db.Query(`SELECT e.id FROM entries e INNER JOIN entry_team et ON et.entry_id = e.id AND et.team_id = ? WHERE e.user_id = ?`, team.ID, target.ID)
	defer rows.Close()
	for rows.Next() {
		var entryID int
		if err := rows.Scan(&entryID); err != nil {
			// TODO
			return
		}
		entryIDs = append(entryIDs, entryID)
	}

	params := append([]interface{}{target.ID}, entryIDs...)
	var query = fmt.Sprintf(`DELETE FROM votes WHERE user_id = ? AND entry_id IN (%s) `, strings.Join(strings.Split(strings.Repeat("?", len(entryIDs)), ""), ","))
	db.Exec(query, params...)

	query = fmt.Sprintf(`DELETE FROM entry_team WHERE entry_id IN (%s) `, strings.Join(strings.Split(strings.Repeat("?", len(entryIDs)), ""), ","))
	db.Exec(query, entryIDs...)

	query = fmt.Sprintf(`DELETE FROM entries WHERE id IN (%s) `, strings.Join(strings.Split(strings.Repeat("?", len(entryIDs)), ""), ","))
	db.Exec(query, entryIDs...)

	enc.Encode(map[string]string{
		"status": "success",
	})
}

var (
	ErrNameExists  = errors.New("the team name already exists")
	ErrNameMissing = errors.New("the team name is missing")
)

func TeamSetAccessKey(ctx context.Context, w http.ResponseWriter, req *http.Request) {
	var db = ctx.Value(DBKey).(*sql.DB)
	var user = ctx.Value(UserKey).(*dash.User)
	var team = ctx.Value(TeamKey).(*dash.Team)

	var enc = json.NewEncoder(w)
	var dec = json.NewDecoder(req.Body)

	var payload map[string]interface{}
	dec.Decode(&payload)

	if team.OwnerID != user.ID {
		enc.Encode(map[string]string{
			"status": "error",
		})
	}

	team.ChangeAccessKey(payload["access_key"].(string))
	if _, err := db.Exec(`UPDATE teams SET access_key = ? WHERE id = ?`, team.EncryptedAccessKey, team.ID); err != nil {
		enc.Encode(map[string]string{
			"status": "error",
		})
		return
	}

	enc.Encode(map[string]string{
		"status": "success",
	})
}

type teamMembershipsResponse struct {
	Status       string            `json:"status"`
	Members      []dash.Membership `json:"members"`
	HasAccessKey bool              `json:"has_access_key"`
}

func TeamListMember(ctx context.Context, w http.ResponseWriter, req *http.Request) {
	var db = ctx.Value(DBKey).(*sql.DB)
	var user = ctx.Value(UserKey).(*dash.User)
	var team = ctx.Value(TeamKey).(*dash.Team)

	var enc = json.NewEncoder(w)
	var dec = json.NewDecoder(req.Body)

	var payload map[string]interface{}
	dec.Decode(&payload)

	if team.OwnerID != user.ID {
		enc.Encode(map[string]string{
			"status": "error",
		})
		return
	}

	var rows *sql.Rows
	var err error
	rows, err = db.Query(`SELECT tm.role, u.username FROM team_user AS tm INNER JOIN users AS u ON u.id = tm.user_id WHERE tm.team_id = ?`, team.ID)
	defer rows.Close()
	if err != nil {
		return
	}

	var memberships = make([]dash.Membership, 0)
	for rows.Next() {
		var membership = dash.Membership{}
		if err := rows.Scan(&membership.Role, &membership.Username); err != nil {
			return
		}
		memberships = append(memberships, membership)
	}

	if err != nil {
		enc.Encode(map[string]string{
			"status": "error",
		})
		return
	}

	var resp = teamMembershipsResponse{
		Status:       "success",
		Members:      memberships,
		HasAccessKey: team.EncryptedAccessKey != "",
	}
	enc.Encode(resp)
}
