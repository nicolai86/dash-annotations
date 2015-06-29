package dash

const (
	VoteDown = -1
	VoteUp   = 1
)

type Vote struct {
	ID      int
	Type    int
	EntryID int
	UserID  int
}

func (v *Vote) IsUpvote() bool {
	return v.Type == VoteUp
}
func (v *Vote) IsDownvote() bool {
	return v.Type == VoteDown
}
