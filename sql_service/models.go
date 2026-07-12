package sql_service

import "time"

// User model represents a user in the system
type User struct {
	ID         uint   `gorm:"primaryKey"`
	Username   string `gorm:"uniqueIndex;size:128"`
	Password   string
	Role       string `gorm:"size:32;default:'user'"`                // user, admin, teacher
	Group      Group  `gorm:"foreignKey:GroupName;references:Name;"` // User group
	GroupName  string
	Rating     int `gorm:"default:1500"` // User rating, default 1500
	CreatedAt  time.Time
	CreatedBy  string     `gorm:"size:128"`      // Username of the creator
	Approved   bool       `gorm:"default:false"` // Whether the user is approved by creator
	ApprovedAt *time.Time // When the user was approved
	ApprovedBy string     `gorm:"size:128"` // Who approved the user
}

type Group struct {
	ID              uint   `gorm:"primaryKey"`
	Name            string `gorm:"uniqueIndex;size:128"`
	EditPermission  bool
	UserPermission  bool
	GroupPermission bool
	CreatedAt       time.Time
	CreatedBy       string `gorm:"size:128"` // Username of the creator
}

// Permission model represents a structured permission type
// type Permission struct {
// 	ID   uint   `gorm:"primaryKey"`
// 	Name string `gorm:"uniqueIndex;size:128"` // Permission name, e.g., edit_problems
// }

// Submission model represents a code submission
type Submission struct {
	ID          uint   `gorm:"primaryKey"`
	Username    string `gorm:"index;size:128"`
	ProblemID   uint   `gorm:"index"`
	Code        string `gorm:"type:text"`
	Status      string `gorm:"size:32"` // queued, running, ok, wa, tle, mle, compile_error, runtime_error
	Score       int    // Total score obtained
	MaxMemoryKB int    // Maximum memory usage in KB
	MaxTimeMs   int    // Maximum time usage in ms
	CreatedAt   time.Time
	UpdatedAt   time.Time
	TestResults []TestResult `gorm:"foreignKey:SubmissionID"`
}

// TestResult model represents the result of a test case
type TestResult struct {
	ID           uint `gorm:"primaryKey"`
	SubmissionID uint `gorm:"index"`
	TestIndex    int  `gorm:"column:test_index"`
	Passed       bool
	Output       string `gorm:"type:text"`
	TimeMs       int
	MemoryKB     int
	Status       string `gorm:"size:32"`
	Score        int    // Score for this test case
}

// Problem model represents a coding problem
type Problem struct {
	ID uint `gorm:"primaryKey"`
	// Name        string `gorm:"uniqueIndex;size:128"`
	Title          string `gorm:"size:256"`
	Description    string `gorm:"type:text"`
	TestsCount     int
	TimeLimitMs    int
	MemLimitMB     int
	ProblemVisible bool `gorm:"default:false"` // If false, non-editors cannot view the problem until contest starts
	TestVisible    bool `gorm:"default:false"` // If false, non-editors cannot see evaluation info (test results, scores, etc.)
}

// Contest represents a contest with an associated problem set and a leaderboard.
type Contest struct {
	ID uint `gorm:"primaryKey"`
	// Name        string    `gorm:"uniqueIndex;size:128"`
	Title       string    `gorm:"size:256"`
	Description string    `gorm:"type:text"`
	StartAt     time.Time `gorm:"index"`
	EndAt       time.Time `gorm:"index"`
	Groups      []Group   `gorm:"many2many:contest_groups;"`
	// Type        string    `gorm:"size:32"` // NOI, IOI etc.
	Problems      []Problem `gorm:"many2many:contest_problems;"`
	CreatedBy     string    `gorm:"size:128"`
	CreatedAt     time.Time
	RatingSettled bool `gorm:"default:false"` // Marked true when rating settlement is complete (only once, even on failure)
}

// ContestRatingHistory records rating changes after a contest ends.
type ContestRatingHistory struct {
	ID           uint   `gorm:"primaryKey"`
	Username     string `gorm:"index;size:128"`
	ContestID    uint   `gorm:"index"`
	ContestName  string `gorm:"size:256"`
	Rank         int    `json:"rank"`          // Final rank in contest (considering ties)
	TotalScore   int    `json:"total_score"`   // Total score achieved
	RatingBefore int    `json:"rating_before"` // Rating before contest
	RatingAfter  int    `json:"rating_after"`  // Rating after contest
	RatingChange int    `json:"rating_change"` // Rating delta (after - before)
	CreatedAt    time.Time
}
