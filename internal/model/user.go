package model

// UserSettings stores preferences for a Telegram user.
type UserSettings struct {
	UserID int64    `json:"user_id"`
	Topics []string `json:"topics"`
	Active bool     `json:"active"`
}

// Subscription represents a scheduled message subscription.
type Subscription struct {
	UserID int64 `json:"user_id"`
	// cron-like or interval; for simplicity use seconds interval
	IntervalSec int `json:"interval_sec"`
}
