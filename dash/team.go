package dash

import (
	"time"

	"golang.org/x/crypto/bcrypt"
)

type Team struct {
	ID                 int
	Name               string
	EncryptedAccessKey string
	OwnerID            int
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

func (t *Team) ChangeAccessKey(newKey string) {
	if newKey == "" {
		t.EncryptedAccessKey = ""
		return
	}

	var accessKeyBytes = []byte(newKey)
	var hashed, _ = bcrypt.GenerateFromPassword(accessKeyBytes, 10)
	t.EncryptedAccessKey = string(hashed)
}

func (t *Team) AccessKeysMatch(accessKey string) bool {
	if t.EncryptedAccessKey == "" && accessKey == "" {
		return true
	}
	return bcrypt.CompareHashAndPassword([]byte(t.EncryptedAccessKey), []byte(accessKey)) == nil
}
