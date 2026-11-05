package signalapi

// Only the fields the bridge uses are decoded; the API sends many more.

// Envelope wraps every event signal-cli receives. At most one of the message
// fields is set; receipts, typing indicators and the like leave both nil.
type Envelope struct {
	DataMessage *DataMessage `json:"dataMessage,omitempty"`
	SyncMessage *SyncMessage `json:"syncMessage,omitempty"`
}

// DataMessage is a message sent by someone else.
type DataMessage struct {
	Timestamp   int64        `json:"timestamp"`
	Message     string       `json:"message"`
	GroupInfo   *GroupInfo   `json:"groupInfo,omitempty"`
	Attachments []Attachment `json:"attachments,omitempty"`
}

// SyncMessage is sent by our own other devices. When the account is linked
// to a phone, messages typed on the phone arrive as SentMessage, which has
// the same fields as a DataMessage.
type SyncMessage struct {
	SentMessage *DataMessage `json:"sentMessage,omitempty"`
}

// GroupInfo identifies the group a message was posted in.
type GroupInfo struct {
	GroupID   string `json:"groupId"`
	GroupName string `json:"groupName"`
}

// Attachment describes a file attached to a message.
type Attachment struct {
	ID          string `json:"id"`
	ContentType string `json:"contentType"`
}
