package storage

// Storage defines method necessary for a user storage implementation
import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/russross/blackfriday"

	"dash"
)

type Storage interface {
	Store(*dash.Entry, dash.User) error
	FindPublicByIdentifier(dash.IdentifierDict, *dash.User) ([]dash.Entry, error)
	FindOwnByIdentifier(dash.IdentifierDict, *dash.User) ([]dash.Entry, error)
	FindByTeamAndIdentifier(dash.IdentifierDict, dash.User) ([]dash.Entry, error)
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

var (
	ErrMissingTitle              = errors.New("title is missing")
	ErrMissingBody               = errors.New("body is missing")
	ErrMissingAnchor             = errors.New("anchor is missing")
	ErrPublicAnnotationForbidden = errors.New("public annotations forbidden")
	ErrUpdateForbidden           = errors.New("you are not the author")
)

func (storage *sqlStorage) FindByTeamAndIdentifier(identifier dash.IdentifierDict, user dash.User) ([]dash.Entry, error) {
	if len(user.TeamMemberships) < 1 {
		return nil, nil
	}

	if err := storage.upsertIdentifier(&identifier); err != nil {
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
	var rows, err = storage.db.Query(query, params...)
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

func (storage *sqlStorage) FindPublicByIdentifier(identifier dash.IdentifierDict, user *dash.User) ([]dash.Entry, error) {
	if err := storage.upsertIdentifier(&identifier); err != nil {
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
	var rows, err = storage.db.Query(query, params...)
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

func (storage *sqlStorage) FindOwnByIdentifier(identifier dash.IdentifierDict, user *dash.User) ([]dash.Entry, error) {
	if err := storage.upsertIdentifier(&identifier); err != nil {
		return nil, err
	}

	if user == nil {
		return nil, nil
	}

	var rows, err = storage.db.Query(`SELECT id, title, type, anchor, body, body_rendered, score, user_id FROM entries WHERE user_id = ?`, user.ID)
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

func (storage *sqlStorage) Store(entry *dash.Entry, author dash.User) error {
	if entry.Title == "" {
		return ErrMissingTitle
	}
	if entry.Body == "" {
		return ErrMissingBody
	}
	if entry.Anchor == "" {
		return ErrMissingAnchor
	}
	if err := storage.upsertIdentifier(&entry.Identifier); err != nil {
		return err
	}
	if entry.Public && entry.Identifier.BannedFromPublic {
		return ErrPublicAnnotationForbidden
	}
	var createVote = false
	if entry.ID != 0 && !author.Moderator {
		if err := storage.db.QueryRow(`SELECT user_id FROM entries WHERE id = ?`, entry.ID).Scan(&entry.UserID); err != nil {
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
		var _, err = storage.db.Exec(`UPDATE entries SET
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
		var res, err = storage.db.Exec(`INSERT INTO entries (title, body, body_rendered, type, identifier_id, anchor, public, removed_from_public, score, user_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
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
		storage.db.QueryRow(`SELECT id FROM teams WHERE name = ? LIMIT 1`, t).Scan(&teamID)
		storage.db.Exec(`INSERT INTO entry_team (entry_id, team_id) VALUES (?, ?)`, entry.ID, teamID)
	}

	return nil
}

func (storage sqlStorage) upsertIdentifier(dict *dash.IdentifierDict) error {
	if dict.DocsetFilename == "Mono" && dict.HttrackSource != "" {
		storage.db.QueryRow(`SELECT id FROM identifiers WHERE docset_filename = ? AND httrack_source = ? LIMIT 1`, dict.DocsetFilename, dict.HttrackSource).Scan(&dict.ID)
	} else {
		storage.db.QueryRow(`SELECT id FROM identifiers WHERE docset_filename = ? AND page_path = ? LIMIT 1`, dict.DocsetFilename, dict.PagePath).Scan(&dict.ID)
	}

	if dict.ID == 0 {
		var res, err = storage.db.Exec(`INSERT INTO identifiers
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
