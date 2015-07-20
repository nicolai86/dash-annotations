package dash

import "time"

// Identifier represents the location within docsets. It's used to allow annotations to reference
// a specific location
type Identifier struct {
	ID               int       `json:"-"`
	BannedFromPublic bool      `json:"-"`
	DocsetName       string    `json:"docset_name"`
	DocsetFilename   string    `json:"docset_filename"`
	DocsetPlatform   string    `json:"docset_platform"`
	DocsetBundle     string    `json:"docset_bundle"`
	DocsetVersion    string    `json:"docset_version"`
	PagePath         string    `json:"page_path"`
	PageTitle        string    `json:"page_title"`
	HttrackSource    string    `json:"httrack_source"`
	UpdatedAt        time.Time `json:"updated_at"`
	CreatedAt        time.Time `json:"created_at"`
}

func (dict Identifier) IsEmpty() bool {
	return dict.DocsetName == "" && dict.DocsetFilename == "" && dict.DocsetPlatform == "" && dict.DocsetBundle == "" && dict.DocsetVersion == ""
}
