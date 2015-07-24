package main

import (
	"database/sql"
	"time"

	"dash"

	"golang.org/x/crypto/bcrypt"
)

// UserTokenUpdater takes a username and a token and updates the storage
type UserTokenUpdater interface {
	UpdateUserWithToken(username, token string) error
}

// UserFinderByUsername retrieves a user by username from the storage
type UserFinderByUsername interface {
	FindUserByUsername(username string) (dash.User, error)
}

// UserPasswordUpdater updates a user with a new plaintext password
type UserPasswordUpdater interface {
	UpdateUserWithPassword(username, password string) error
}

// UserEmailUpdater updates a user with a new email
type UserEmailUpdater interface {
	UpdateUserWithEmail(username, email string) error
}

// UserCreater stores a new user in the storage
type UserCreater interface {
	InsertUser(username, password string) error
}

type sqlUserStorage struct {
	db *sql.DB
}

func encryptPassword(plainPassword string) string {
	var passwordBytes = []byte(plainPassword)
	var hashed, _ = bcrypt.GenerateFromPassword(passwordBytes, 10)
	return string(hashed)
}

func (store *sqlUserStorage) FindUserByUsername(username string) (dash.User, error) {
	return findUserByCondition(store.db, `username = ?`, username)
}

func (store *sqlUserStorage) UpdateUserWithToken(username, token string) error {
	var _, err = store.db.Exec(`UPDATE users SET remember_token = ?, updated_at = ? WHERE username = ?`, token, time.Now(), username)
	return err
}

func (store *sqlUserStorage) UpdateUserWithPassword(username, password string) error {
	var _, err = store.db.Exec(`UPDATE users SET password = ?, updated_at = ? WHERE username = ?`, encryptPassword(password), time.Now(), username)
	return err
}

func (store *sqlUserStorage) InsertUser(username, password string) error {
	var existingUserID = -1
	store.db.QueryRow(`SELECT id FROM users WHERE username = ?`, username).Scan(&existingUserID)
	if existingUserID != -1 {
		return ErrUsernameExists
	}

	if _, err := store.db.Exec(`INSERT INTO users (username, password, created_at, updated_at) VALUES (?, ?, ?, ?)`, username, encryptPassword(password), time.Now(), time.Now()); err != nil {
		return err
	}

	return nil
}

func (store *sqlUserStorage) UpdateUserWithEmail(username, email string) error {
	var existingUserID = -1
	store.db.QueryRow(`SELECT id FROM users WHERE email = ? AND username != ?`, email, username).Scan(&existingUserID)
	if existingUserID != -1 {
		return ErrEmailExists
	}

	if _, err := store.db.Exec(`UPDATE users SET email = ?, updated_at = ? WHERE username = ?`, email, time.Now(), username); err != nil {
		return err
	}

	return nil
}

func findUserByUsername(db *sql.DB, username string) (dash.User, error) {
	return findUserByCondition(db, `username = ?`, username)
}

func findUserByRememberToken(db *sql.DB, token string) (dash.User, error) {
	return findUserByCondition(db, `remember_token = ?`, token)
}

func findUserByCondition(db *sql.DB, cond string, param interface{}) (dash.User, error) {
	var user = dash.User{}
	if err := db.QueryRow(`SELECT id, username, email, password, remember_token, moderator FROM users WHERE `+cond, param).Scan(&user.ID, &user.Username, &user.Email, &user.EncryptedPassword, &user.RememberToken, &user.Moderator); err != nil {
		return user, err
	}

	var rows, err = db.Query(`SELECT t.id, t.name, tm.role FROM team_user AS tm INNER JOIN teams AS t ON t.id = tm.team_id WHERE tm.user_id = ?`, user.ID)
	if err != nil {
		return user, err
	}
	defer rows.Close()

	var memberships = make([]dash.TeamMember, 0)
	for rows.Next() {
		var membership = dash.TeamMember{}
		if err := rows.Scan(&membership.TeamID, &membership.TeamName, &membership.Role); err != nil {
			return user, err
		}
		memberships = append(memberships, membership)
	}

	user.TeamMemberships = memberships
	return user, nil
}
