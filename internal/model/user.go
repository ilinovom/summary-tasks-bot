package model

// UserSettings stores preferences for a Telegram user.
type UserSettings struct {
	UserID            int64               `json:"user_id"`
	UserName          string              `json:"username"`
	Active            bool                `json:"active"`
	Topics            map[string][]string `json:"topics,omitempty"`
	Frequency         int                 `json:"frequency,omitempty"`
	Tariff            string              `json:"tariff,omitempty"`
	LastScheduledSent int64               `json:"last_scheduled_sent,omitempty"`
	LastGetNewsNow    int64               `json:"last_get_news_now,omitempty"`
	GetNewsNowCount   int                 `json:"get_news_now_count,omitempty"`
	LastGetLast24h    int64               `json:"last_get_last_24h,omitempty"`
	GetLast24hCount   int                 `json:"get_last_24h_count,omitempty"`
}

// Subscription represents a scheduled message subscription.
type Subscription struct {
	UserID int64 `json:"user_id"`
	// cron-like or interval; for simplicity use seconds interval
	IntervalSec int `json:"interval_sec"`
}
