package status

import "time"

// SessionStatusEvent represents a progress update for a translation session.
type SessionStatusEvent struct {
	SessionID string    `json:"sessionId"`
	Stage     string    `json:"stage"`
	State     string    `json:"state"`
	Detail    string    `json:"detail,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

func channelName(sessionID string) string {
	return "streamlation:session:" + sessionID + ":status"
}
