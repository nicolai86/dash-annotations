package dash

import "time"

type Entry struct {
	ID                int        `json:"id"`
	Title             string     `json:"title"`
	Body              string     `json:"-"`
	BodyRendered      string     `json:"-"`
	Public            bool       `json:"public"`
	Type              string     `json:"type"`
	Teams             []string   `json:"teams"`
	Identifier        Identifier `json:"-"`
	IdentifierID      int        `json:"-"`
	Anchor            string     `json:"anchor"`
	UserID            int        `json:"-"`
	AuthorUsername    string     `json:"-"`
	Score             int        `json:"score"`
	RemovedFromPublic bool       `json:"-"`
	UpdatedAt         time.Time  `json:"updated_at"`
	CreatedAt         time.Time  `json:"created_at"`
}
