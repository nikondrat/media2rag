package model

type Event struct {
	Type     string      `json:"type"`
	Data     interface{} `json:"data,omitempty"`
	Progress float64     `json:"progress,omitempty"`
	Error    string      `json:"error,omitempty"`
}
