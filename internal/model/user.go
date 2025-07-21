package model

// UserSettings stores preferences for a Telegram user.
type UserSettings struct {
	UserID     int64    `json:"user_id"`
	Topics     []string `json:"topics"`
	Active     bool     `json:"active"`
	InfoTypes  []string `json:"info_types,omitempty"`
	Categories []string `json:"categories,omitempty"`
	Frequency  int      `json:"frequency,omitempty"`
	Tariff     string   `json:"tariff,omitempty"`
}

// Subscription represents a scheduled message subscription.
type Subscription struct {
	UserID int64 `json:"user_id"`
	// cron-like or interval; for simplicity use seconds interval
	IntervalSec int `json:"interval_sec"`
}
