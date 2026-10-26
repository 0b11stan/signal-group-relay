package signalapi

// Incoming is one event pushed by signal-cli-rest-api on the receive
// websocket.
type Incoming struct {
	Envelope Envelope `json:"envelope"`
	Account  string   `json:"account"`
}

// Envelope wraps every event signal-cli receives. Exactly one of the message
// fields is usually set.
type Envelope struct {
	Source       string `json:"source"`
	SourceNumber string `json:"sourceNumber"`
	SourceUUID   string `json:"sourceUuid"`
	SourceName   string `json:"sourceName"`
	SourceDevice int    `json:"sourceDevice"`
	Timestamp    int64  `json:"timestamp"`

	DataMessage    *DataMessage    `json:"dataMessage,omitempty"`
	SyncMessage    *SyncMessage    `json:"syncMessage,omitempty"`
	ReceiptMessage *ReceiptMessage `json:"receiptMessage,omitempty"`
	TypingMessage  *TypingMessage  `json:"typingMessage,omitempty"`
}

// DataMessage is a message sent by someone else.
type DataMessage struct {
	Timestamp   int64        `json:"timestamp"`
	Message     string       `json:"message"`
	GroupInfo   *GroupInfo   `json:"groupInfo,omitempty"`
	Attachments []Attachment `json:"attachments,omitempty"`
}

// SyncMessage is sent by our own other devices. When the account is linked
// to a phone, messages typed on the phone arrive as SentMessage.
type SyncMessage struct {
	SentMessage *SentMessage `json:"sentMessage,omitempty"`
}

// SentMessage is a message sent from another device of our own account.
type SentMessage struct {
	Destination       string       `json:"destination"`
	DestinationNumber string       `json:"destinationNumber"`
	Timestamp         int64        `json:"timestamp"`
	Message           string       `json:"message"`
	GroupInfo         *GroupInfo   `json:"groupInfo,omitempty"`
	Attachments       []Attachment `json:"attachments,omitempty"`
}

// GroupInfo identifies the group a message was posted in.
type GroupInfo struct {
	GroupID   string `json:"groupId"`
	GroupName string `json:"groupName"`
	Type      string `json:"type"`
}

// Attachment describes a file attached to a message.
type Attachment struct {
	ID          string `json:"id"`
	ContentType string `json:"contentType"`
	Filename    string `json:"filename"`
	Size        int64  `json:"size"`
}

// ReceiptMessage is a delivery or read receipt.
type ReceiptMessage struct {
	When       int64   `json:"when"`
	IsDelivery bool    `json:"isDelivery"`
	IsRead     bool    `json:"isRead"`
	Timestamps []int64 `json:"timestamps"`
}

// TypingMessage is a typing indicator.
type TypingMessage struct {
	Action    string `json:"action"`
	Timestamp int64  `json:"timestamp"`
	GroupID   string `json:"groupId"`
}
