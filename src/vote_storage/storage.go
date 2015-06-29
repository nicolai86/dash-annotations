package storage

// Storage defines method necessary for a user storage implementation
import (
	"database/sql"
	"time"

	"dash"
)

type Storage interface {
	Upsert(*dash.Vote) error
	FindVoteByEntryAndUser(dash.Entry, dash.User) (dash.Vote, error)
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

func (storage *sqlStorage) Upsert(vote *dash.Vote) error {
	if vote.ID != 0 {
		var _, err = storage.db.Exec(`UPDATE votes SET type = ?, updated_at = ? WHERE entry_id = ? AND user_id = ?`, vote.Type, time.Now(), vote.EntryID, vote.UserID)
		return err
	}

	var res, err = storage.db.Exec(`INSERT INTO votes (type, entry_id, user_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`, vote.Type, vote.EntryID, vote.UserID, time.Now(), time.Now())
	var voteID int64
	voteID, err = res.LastInsertId()
	vote.ID = int(voteID)
	return err
}

func (storage *sqlStorage) FindVoteByEntryAndUser(entry dash.Entry, u dash.User) (dash.Vote, error) {
	var vote = dash.Vote{
		EntryID: entry.ID,
		UserID:  u.ID,
	}
	var err = storage.db.QueryRow(`SELECT id, type, entry_id, user_id FROM votes WHERE entry_id = ? AND user_id = ?`, entry.ID, u.ID).Scan(&vote.ID, &vote.Type, &vote.EntryID, &vote.UserID)
	return vote, err
}
