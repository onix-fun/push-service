package model

type Device struct {
	ID          string `json:"id"`
	RecipientID string `json:"recipient_id"`
	Provider    string `json:"provider"`
	Token       string `json:"token"`
	Active      bool   `json:"active"`
}

type Command struct {
	MessageID   string         `json:"message_id"`
	RecipientID string         `json:"recipient_id"`
	Title       string         `json:"title"`
	Body        string         `json:"body"`
	Data        map[string]any `json:"data,omitempty"`
	TTL         int            `json:"ttl,omitempty"`
	CollapseKey string         `json:"collapse_key,omitempty"`
}

type DeliveryEvent struct {
	MessageID   string `json:"message_id"`
	DeviceID    string `json:"device_id"`
	RecipientID string `json:"recipient_id"`
	Provider    string `json:"provider"`
	Status      string `json:"status"`
	Error       string `json:"error,omitempty"`
}
