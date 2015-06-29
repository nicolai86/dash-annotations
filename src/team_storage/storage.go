package storage

// Storage defines method necessary for a user storage implementation
import (
	"database/sql"
	"errors"
	"time"

	"dash"
)

type Storage interface {
	Store(*dash.Team, dash.User) error
	AddMembership(dash.TeamMember) error
	RemoveMembership(dash.TeamMember) error
	UpdateMembership(dash.TeamMember) error
	FindTeamByName(string) (dash.Team, error)
	FindTeamMembershipsForUser(dash.User) ([]dash.TeamMember, error)
	FindTeamMembershipsForTeam(*dash.Team) ([]dash.Membership, error)
}

type sqlStorage struct {
	db *sql.DB
}

var (
	ErrNameExists  = errors.New("the team name already exists")
	ErrNameMissing = errors.New("the team name is missing")
)

// New returns a new instance of the sqlStorage for users
func New(db *sql.DB) Storage {
	return &sqlStorage{
		db: db,
	}
}

func (storage sqlStorage) AddMembership(membership dash.TeamMember) error {
	var _, err = storage.db.Exec(`INSERT INTO team_user (team_id, user_id, role) VALUES (?, ?, ?)`, membership.TeamID, membership.UserID, membership.Role)
	return err
}

func (storage sqlStorage) RemoveMembership(membership dash.TeamMember) error {
	var _, err = storage.db.Exec(`DELETE FROM team_user WHERE team_id = ? AND user_id = ?`, membership.TeamID, membership.UserID)
	return err
}

func (storage sqlStorage) UpdateMembership(membership dash.TeamMember) error {
	var _, err = storage.db.Exec(`UPDATE team_user SET role = ? WHERE team_id = ? AND user_id = ?`, membership.Role, membership.TeamID, membership.UserID)
	return err
}

func (storage sqlStorage) FindTeamByName(name string) (dash.Team, error) {
	var team = dash.Team{
		Name: name,
	}
	if err := storage.db.QueryRow(`SELECT t.id, t.access_key, tm.user_id FROM teams AS t INNER JOIN team_user AS tm ON tm.team_id = t.id WHERE name = ? AND tm.role = ? LIMIT 1`, name, "owner").Scan(&team.ID, &team.EncryptedAccessKey, &team.OwnerID); err != nil {
		return team, err
	}
	return team, nil
}

func (storage sqlStorage) Store(team *dash.Team, owner dash.User) error {
	if team.Name == "" {
		return ErrNameMissing
	}

	var isNewTeam = team.ID == 0
	if isNewTeam {
		var cnt = 0
		var err = storage.db.QueryRow(`SELECT count(*) FROM teams WHERE name = ?`, team.Name).Scan(&cnt)
		if err != nil {
			return err
		}
		if cnt != 0 {
			return ErrNameExists
		}
		var ins, insErr = storage.db.Exec(`INSERT INTO teams (name, created_at, updated_at) VALUES (?, ?, ?)`, team.Name, time.Now(), time.Now())
		var teamID int64
		teamID, _ = ins.LastInsertId()
		team.ID = int(teamID)

		storage.db.Exec(`INSERT INTO team_user (team_id, user_id, role) VALUES (?, ?, ?)`, team.ID, owner.ID, "owner")
		return insErr
	}

	storage.db.Exec(`UPDATE teams SET access_key = ? WHERE id = ?`, team.EncryptedAccessKey, team.ID)
	return nil
}

func (storage sqlStorage) FindTeamMembershipsForTeam(team *dash.Team) ([]dash.Membership, error) {
	var rows, err = storage.db.Query(`SELECT tm.role, u.username FROM team_user AS tm INNER JOIN users AS u ON u.id = tm.user_id WHERE tm.team_id = ?`, team.ID)
	defer rows.Close()
	if err != nil {
		return nil, err
	}

	var memberships = make([]dash.Membership, 0)
	for rows.Next() {
		var membership = dash.Membership{}
		if err := rows.Scan(&membership.Role, &membership.Username); err != nil {
			return nil, err
		}
		memberships = append(memberships, membership)
	}

	return memberships, nil
}

func (storage sqlStorage) FindTeamMembershipsForUser(u dash.User) ([]dash.TeamMember, error) {
	var rows, err = storage.db.Query(`SELECT t.id, t.name, tm.role FROM team_user AS tm INNER JOIN teams AS t ON t.id = tm.team_id WHERE tm.user_id = ?`, u.ID)
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
