package service

type (
	// StatusChangeMsg describes status change event message body
	StatusChangeMsg struct {
		AlarmID   string `json:"AlarmID"    validate:"required,uuid"`
		UserID    string `json:"UserID"     validate:"required,uuid"`
		Status    string `json:"Status"     validate:"required,alarm_status"`
		ChangedAt string `json:"ChangedAt"`
	}

	// AlarmDigestMsg describes alarm digest event msg body
	AlarmDigestMsg struct {
		UserID       string        `json:"UserID"`
		ActiveAlarms []ActiveAlarm `json:"ActiveAlarms"`
	}

	// ActiveAlarm active alarms
	ActiveAlarm struct {
		AlarmID         string `json:"AlarmID"`
		Status          string `json:"Status"`
		LatestChangedAt string `json:"LatestChangedAt"`
	}

	// SendDigestMsg describes send digest event msg body
	SendDigestMsg struct {
		UserID string `json:"UserID" validate:"required,uuid"`
	}
)
