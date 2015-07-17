package dash

import (
	"database/sql"

	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID                int
	Username          string
	Email             sql.NullString
	EncryptedPassword string
	RememberToken     sql.NullString
	TeamMemberships   []TeamMember
	Moderator         bool
}

func (u *User) ChangePassword(newPassword string) {
	var passwordBytes = []byte(newPassword)
	var hashed, _ = bcrypt.GenerateFromPassword(passwordBytes, 10)
	u.EncryptedPassword = string(hashed)
}

func (u *User) PasswordsMatch(password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(u.EncryptedPassword), []byte(password)) == nil
}

type TeamMember struct {
	TeamID   int    `json:"-"`
	TeamName string `json:"name"`
	Role     string `json:"role"`
	UserID   int    `json:"-"`
}
