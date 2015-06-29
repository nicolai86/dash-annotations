package dash

import "time"

type Entry struct {
	ID                int      `json:"id"`
	Title             string   `json:"title"`
	Body              string   `json:"-"`
	BodyRendered      string   `json:"-"`
	Public            bool     `json:"public"`
	Type              string   `json:"type"`
	Teams             []string `json:"teams"`
	Identifier        IdentifierDict
	IdentifierID      int       `json:"-"`
	Anchor            string    `json:"anchor"`
	UserID            int       `json:"-"`
	AuthorUsername    string    `json:"-"`
	Score             int       `json:"score"`
	UpdatedAt         time.Time `json:"updated_at"`
	CreatedAt         time.Time `json:"created_at"`
	RemovedFromPublic bool      `json:"-"`
}

type IdentifierDict struct {
	ID               int    `json:"-"`
	BannedFromPublic bool   `json:"-"`
	DocsetName       string `json:"docset_name"`
	DocsetFilename   string `json:"docset_filename"`
	DocsetPlatform   string `json:"docset_platform"`
	DocsetBundle     string `json:"docset_bundle"`
	DocsetVersion    string `json:"docset_version"`
	PagePath         string `json:"page_path"`
	PageTitle        string `json:"page_title"`
	HttrackSource    string `json:"httrack_source"`
}

func (dict IdentifierDict) IsEmpty() bool {
	return false
}
