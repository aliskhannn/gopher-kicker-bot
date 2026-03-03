package models

import "time"

type UserState struct {
	UserID     int64     `json:"user_id"`
	ChatID     int64     `json:"chat_id"`
	State      string    `json:"state"` // "waiting_answer"
	Task       string    `json:"task"`
	Answer     int       `json:"answer"`
	Username   string    `json:"username,omitempty"`
	SentAt     time.Time `json:"sent_at"`
	WarnAt     time.Time `json:"warn_at"`
	TimeoutAt  time.Time `json:"timeout_at"`
	WarnSentAt time.Time `json:"warn_sent_at,omitempty"`
	QuestionID int       `json:"question_id"`
}
