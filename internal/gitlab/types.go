package gitlab

// PushEvent is the normalized representation of a GitLab push webhook payload.
type PushEvent struct {
	ObjectKind  string   `json:"object_kind"`
	Ref         string   `json:"ref"`   // "refs/heads/RIS-123-fix-something"
	Before      string   `json:"before"` // all zeros means new branch
	After       string   `json:"after"`
	ProjectID   int      `json:"project_id"`
	UserEmail   string   `json:"user_email"`
	UserName    string   `json:"user_name"`
	Commits     []Commit `json:"commits"`
}

type Commit struct {
	ID      string `json:"id"`
	Message string `json:"message"`
	Author  struct {
		Email string `json:"email"`
	} `json:"author"`
}

// MergeRequestEvent is the normalized representation of a GitLab merge_request webhook payload.
type MergeRequestEvent struct {
	ObjectKind       string                `json:"object_kind"`
	User             User                  `json:"user"`
	Project          Project               `json:"project"`
	ObjectAttributes MRObjectAttributes    `json:"object_attributes"`
}

type User struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

type Project struct {
	ID int `json:"id"`
}

type MRObjectAttributes struct {
	IID            int    `json:"iid"`
	Title          string `json:"title"`
	SourceBranch   string `json:"source_branch"`
	TargetBranch   string `json:"target_branch"`
	State          string `json:"state"`
	Action         string `json:"action"`           // "open", "merge", "close", "update"
	Draft          bool   `json:"draft"`             // true if this is a draft/WIP MR
	WorkInProgress bool   `json:"work_in_progress"` // legacy field, same as Draft
	AuthorID       int    `json:"author_id"`
}

// EmojiEvent is the normalized representation of a GitLab emoji webhook payload.
type EmojiEvent struct {
	ObjectKind       string          `json:"object_kind"`
	EventType        string          `json:"event_type"`        // "award" or "revoke" (top-level in GitLab CE)
	User             User            `json:"user"`
	Project          Project         `json:"project"`
	ObjectAttributes EmojiAttributes `json:"object_attributes"`
	MergeRequest     *MRFromEmoji    `json:"merge_request"`
}

type EmojiAttributes struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`           // "thumbsup", "rocket", etc.
	AwardableType string `json:"awardable_type"` // "MergeRequest" or "Note"
	AwardableID   int    `json:"awardable_id"`
}

type MRFromEmoji struct {
	IID          int    `json:"iid"`
	Title        string `json:"title"`
	TargetBranch string `json:"target_branch"`
	State        string `json:"state"`
	SourceBranch string `json:"source_branch"`
}
