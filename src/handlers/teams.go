package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	entryStore "entry_storage"
	teamStore "team_storage"
	userStore "user_storage"

	"dash"
)

type TeamsHandler struct {
	UserStorage  userStore.Storage
	TeamStorage  teamStore.Storage
	EntryStorage entryStore.Storage
}

type teamListResponse struct {
	Status string            `json:"status"`
	Teams  []dash.TeamMember `json:"teams,omitempty"`
}

type teamMembershipsResponse struct {
	Status       string            `json:"status"`
	Members      []dash.Membership `json:"members"`
	HasAccessKey bool              `json:"has_access_key"`
}

func (th *TeamsHandler) list(w http.ResponseWriter, req *http.Request) {
	var u, err = getUserFromSession(th.UserStorage, req)
	var enc = json.NewEncoder(w)
	if err != nil {
		enc.Encode(map[string]string{
			"status":  "error",
			"message": "Error. Logout and try again",
		})
		return
	}

	var teams, fetchErr = th.TeamStorage.FindTeamMembershipsForUser(u)
	if fetchErr != nil {
		enc.Encode(map[string]string{
			"status": "error",
		})
		return
	}

	var resp = teamListResponse{
		Status: "success",
		Teams:  teams,
	}
	enc.Encode(resp)
}

type teamCreateRequest struct {
	Name string `json:"name"`
}

func (th *TeamsHandler) create(w http.ResponseWriter, req *http.Request) {
	var u, err = getUserFromSession(th.UserStorage, req)
	var enc = json.NewEncoder(w)
	if err != nil {
		enc.Encode(map[string]string{
			"status":  "error",
			"message": "Error. Logout and try again",
		})
		return
	}

	var dec = json.NewDecoder(req.Body)
	var payload teamCreateRequest
	dec.Decode(&payload)
	var team = dash.Team{
		Name: payload.Name,
	}

	if err := th.TeamStorage.Store(&team, u); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		switch err {
		case teamStore.ErrNameExists:
			enc.Encode(map[string]string{
				"status":  "error",
				"message": "Team name already taken",
			})
		case teamStore.ErrNameMissing:
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

	enc.Encode(map[string]string{
		"status": "success",
	})
}

type teamJoinRequest struct {
	Name      string `json:"name"`
	AccessKey string `json:"access_key"`
}

func (th *TeamsHandler) join(w http.ResponseWriter, req *http.Request) {
	var u, err = getUserFromSession(th.UserStorage, req)
	var enc = json.NewEncoder(w)
	if err != nil {
		enc.Encode(map[string]string{
			"status":  "error",
			"message": "Error. Logout and try again",
		})
		return
	}

	var dec = json.NewDecoder(req.Body)
	var payload teamJoinRequest
	dec.Decode(&payload)
	var team dash.Team
	team, err = th.TeamStorage.FindTeamByName(payload.Name)
	if err != nil {
		enc.Encode(map[string]string{
			"status":  "error",
			"message": "Team does not exist",
		})
		return
	}

	if !team.AccessKeysMatch(payload.AccessKey) {
		enc.Encode(map[string]string{
			"status":  "error",
			"message": "Invalid access key",
		})
		return
	}

	if err := th.TeamStorage.AddMembership(dash.TeamMember{
		TeamID: team.ID,
		Role:   "member",
		UserID: u.ID,
	}); err != nil {
		enc.Encode(map[string]string{
			"status":  "error",
			"message": "You are already a member of this team"})
		return
	}

	enc.Encode(map[string]string{
		"status": "success",
	})
}

type teamLeaveRequest teamCreateRequest

func (th *TeamsHandler) leave(w http.ResponseWriter, req *http.Request) {
	var u, err = getUserFromSession(th.UserStorage, req)
	var enc = json.NewEncoder(w)
	if err != nil {
		enc.Encode(map[string]string{
			"status":  "error",
			"message": "Error. Logout and try again",
		})
		return
	}

	var payload teamLeaveRequest
	var dec = json.NewDecoder(req.Body)
	dec.Decode(&payload)

	var team dash.Team
	team, err = th.TeamStorage.FindTeamByName(payload.Name)
	if err != nil {
		enc.Encode(map[string]string{
			"status":  "error",
			"message": "Team does not exist",
		})
		return
	}

	th.TeamStorage.RemoveMembership(dash.TeamMember{
		TeamID: team.ID,
		UserID: u.ID,
	})

	// TODO remove posts

	enc.Encode(map[string]string{
		"status": "success",
	})

}

type teamSetRoleRequest struct {
	TeamName string `json:"name"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

func (th *TeamsHandler) setRole(w http.ResponseWriter, req *http.Request) {
	var u, err = getUserFromSession(th.UserStorage, req)
	var enc = json.NewEncoder(w)
	if err != nil {
		enc.Encode(map[string]string{
			"status":  "error",
			"message": "Error. Logout and try again",
		})
		return
	}

	var payload teamSetRoleRequest
	var dec = json.NewDecoder(req.Body)
	dec.Decode(&payload)

	var team dash.Team
	team, err = th.TeamStorage.FindTeamByName(payload.TeamName)
	if err != nil {
		enc.Encode(map[string]string{
			"status":  "error",
			"message": "Team does not exist",
		})
		return
	}

	if payload.Role == "" || payload.Username == "" {
		enc.Encode(map[string]string{
			"status":  "error",
			"message": "Missing Role | Username",
		})
		return
	}

	if team.OwnerID != u.ID {
		enc.Encode(map[string]string{
			"status":  "error",
			"message": "Error. Need to be owner.",
		})
	}

	var target dash.User
	target, err = th.UserStorage.FindByUsername(payload.Username)
	if err != nil {
		enc.Encode(map[string]string{
			"status":  "error",
			"message": "Unknown user",
		})
		return
	}

	th.TeamStorage.UpdateMembership(dash.TeamMember{
		TeamID: team.ID,
		UserID: target.ID,
		Role:   payload.Role,
	})

	enc.Encode(map[string]string{
		"status": "success",
	})
}

func (th *TeamsHandler) removeMember(w http.ResponseWriter, req *http.Request) {
	var u, err = getUserFromSession(th.UserStorage, req)
	var enc = json.NewEncoder(w)
	if err != nil {
		enc.Encode(map[string]string{
			"status":  "error",
			"message": "Error. Logout and try again",
		})
		return
	}

	var payload teamSetRoleRequest
	var dec = json.NewDecoder(req.Body)
	dec.Decode(&payload)

	var team dash.Team
	team, err = th.TeamStorage.FindTeamByName(payload.TeamName)
	if err != nil {
		enc.Encode(map[string]string{
			"status":  "error",
			"message": "Team does not exist",
		})
		return
	}

	if payload.Username == "" {
		enc.Encode(map[string]string{
			"status":  "error",
			"message": "Missing Role | Username",
		})
		return
	}

	if team.OwnerID != u.ID {
		enc.Encode(map[string]string{
			"status":  "error",
			"message": "Error. Need to be owner.",
		})
	}

	var target dash.User
	target, err = th.UserStorage.FindByUsername(payload.Username)
	if err != nil {
		enc.Encode(map[string]string{
			"status":  "error",
			"message": "Unknown user",
		})
		return
	}

	if err := th.TeamStorage.RemoveMembership(dash.TeamMember{
		TeamID: team.ID,
		UserID: target.ID,
	}); err != nil {
		log.Printf("%v", err)
		enc.Encode(map[string]string{
			"status":  "error",
			"message": "Unknown user",
		})
		return
	}

	// TODO deteach posts

	enc.Encode(map[string]string{
		"status": "success",
	})
}

type teamAccessKeyRequest struct {
	Name      string `json:"name"`
	AccessKey string `json:"access_key"`
}

func (th *TeamsHandler) setAccessKey(w http.ResponseWriter, req *http.Request) {
	var u, err = getUserFromSession(th.UserStorage, req)
	var enc = json.NewEncoder(w)
	if err != nil {
		enc.Encode(map[string]string{
			"status":  "error",
			"message": "Error. Logout and try again",
		})
		return
	}

	var dec = json.NewDecoder(req.Body)
	var payload teamAccessKeyRequest
	dec.Decode(&payload)

	var team dash.Team
	team, err = th.TeamStorage.FindTeamByName(payload.Name)
	if err != nil {
		enc.Encode(map[string]string{
			"status": "error",
		})
		return
	}

	if team.OwnerID != u.ID {
		enc.Encode(map[string]string{
			"status": "error",
		})
	}

	team.ChangeAccessKey(payload.AccessKey)
	if err := th.TeamStorage.Store(&team, u); err != nil {
		enc.Encode(map[string]string{
			"status": "error",
		})
		return
	}

	enc.Encode(map[string]string{
		"status": "success",
	})

}

func (th *TeamsHandler) listMembers(w http.ResponseWriter, req *http.Request) {
	var u, err = getUserFromSession(th.UserStorage, req)
	var enc = json.NewEncoder(w)
	if err != nil {
		enc.Encode(map[string]string{
			"status":  "error",
			"message": "Error. Logout and try again",
		})
		return
	}

	var dec = json.NewDecoder(req.Body)
	var payload teamCreateRequest
	dec.Decode(&payload)

	var team dash.Team
	if team, err = th.TeamStorage.FindTeamByName(payload.Name); err != nil {
		enc.Encode(map[string]string{
			"status": "error",
		})
		return
	}

	if team.OwnerID != u.ID {
		enc.Encode(map[string]string{
			"status": "error",
		})
	}

	var memberships, fetchErr = th.TeamStorage.FindTeamMembershipsForTeam(&team)
	if fetchErr != nil {
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

func (th *TeamsHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	switch req.URL.Path {
	case "list":
		th.list(w, req)
	case "create":
		th.create(w, req)
	case "join":
		th.join(w, req)
	case "leave":
		th.leave(w, req)
	case "set_role":
		th.setRole(w, req)
	case "remove_member":
		th.removeMember(w, req)
	case "set_access_key":
		th.setAccessKey(w, req)
	case "list_members":
		th.listMembers(w, req)
	default:
		log.Printf("Unknown teams route: %v", req.URL.Path)
	}
}
