package dash

const (
	// VoteDown is a negative Vote-Type
	VoteDown = -1
	// VoteUp is a positive Vote-Type
	VoteUp = 1
)

// Vote identifies that a user either up- or downvoted an entry
type Vote struct {
	ID      int
	Type    int
	EntryID int
	UserID  int
}

// IsUpvote is a predicate testing for VoteUp types
func (v *Vote) IsUpvote() bool {
	return v.Type == VoteUp
}

// IsDownvote is a predicate testing for VoteDown types
func (v *Vote) IsDownvote() bool {
	return v.Type == VoteDown
}
