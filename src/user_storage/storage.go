package storage

import (
	"dash"
	"database/sql"
	"log"
	"time"
)

// Storage defines method necessary for a user storage implementation
type Storage interface {
	Store(*dash.User) error
	Update(*dash.User) error
	FindByUsername(string) (dash.User, error)
	FindByEmail(string) (dash.User, error)
	FindByRememberToken(string) (dash.User, error)
}

type sqlStorage struct {
	db *sql.DB
}

// New returns a new instance of the sqlStorage for users
func New(db *sql.DB) Storage {
	return &sqlStorage{
		db: db,
	}
}

func (storage sqlStorage) Store(u *dash.User) error {
	var res, err = storage.db.Exec(`INSERT INTO users (username, password, created_at) VALUES (?, ?, ?)`, u.Username, u.EncryptedPassword, time.Now())
	if err != nil {
		log.Printf("err: %v", err)
		return err
	}
	var userID int64
	userID, err = res.LastInsertId()
	if err != nil {
		log.Printf("err: %v", err)
		return err
	}
	u.ID = int(userID)
	return nil
}

func (storage sqlStorage) Update(u *dash.User) error {
	var query = `UPDATE users SET username = ?, updated_at = ?, password = ?`
	var params = []interface{}{
		u.Username,
		time.Now(),
		u.EncryptedPassword,
	}

	if u.Email.Valid {
		query += `, email = ?`
		params = append(params, u.Email.String)
	}
	if u.RememberToken.Valid {
		query += `, remember_token = ?`
		params = append(params, u.RememberToken.String)
	}

	query += ` WHERE id = ?`
	params = append(params, u.ID)
	var _, err = storage.db.Exec(query, params...)
	if err != nil {
		log.Printf("err: %v", err)
	}
	return err
}

func (storage sqlStorage) FindByRememberToken(token string) (dash.User, error) {
	var user = dash.User{}
	var err = storage.db.QueryRow(`SELECT id, username, email, password, moderator FROM users WHERE remember_token = ?`, token).Scan(&user.ID, &user.Username, &user.Email, &user.EncryptedPassword, &user.Moderator)
	return user, err
}

func (storage sqlStorage) FindByUsername(username string) (dash.User, error) {
	var user = dash.User{
		Username: username,
	}
	var err = storage.db.QueryRow(`SELECT id, email, password, remember_token, moderator FROM users WHERE username = ?`, username).Scan(&user.ID, &user.Email, &user.EncryptedPassword, &user.RememberToken, &user.Moderator)
	return user, err
}

func (storage sqlStorage) FindByEmail(email string) (dash.User, error) {
	var user = dash.User{}
	user.Email = sql.NullString{String: email, Valid: true}
	var err = storage.db.QueryRow(`SELECT id, username, moderator, remember_token, password FROM users WHERE email = ?`, email).Scan(&user.ID, &user.Username, &user.Moderator, &user.RememberToken, &user.EncryptedPassword)
	return user, err
}
