package websocket

import "encoding/json"

// Message represents a WebSocket message
type Message struct {
	Type string `json:"type"`
	Data string `json:"data"`
}

// UnmarshalMessage unmarshals a JSON message
func UnmarshalMessage(data []byte, msg *Message) error {
	return json.Unmarshal(data, msg)
}
