package zernio

import (
	"encoding/json"
	"time"
)

// ─── Inbox Webhook Types ───────────────────────────────────────

// InboxWebhookMessage is the message object in inbox webhook payloads.
type InboxWebhookMessage struct {
	ID                string              `json:"id"`
	ConversationID    string              `json:"conversationId"`
	Platform          string              `json:"platform"`
	PlatformMessageID string              `json:"platformMessageId"`
	Direction         string              `json:"direction"`
	Text              *string             `json:"text"`
	Attachments       []MessageAttachment `json:"attachments"`
	Sender            MessageSender       `json:"sender"`
	SentAt            time.Time           `json:"sentAt"`
	IsRead            bool                `json:"isRead"`
}

type MessageAttachment struct {
	Type    string      `json:"type"`
	URL     string      `json:"url"`
	Payload interface{} `json:"payload,omitempty"`
}

type MessageSender struct {
	ID                         string           `json:"id"`
	ContactID                  *string          `json:"contactId,omitempty"`
	Name                       *string          `json:"name,omitempty"`
	Username                   *string          `json:"username,omitempty"`
	Picture                    *string          `json:"picture,omitempty"`
	PhoneNumber                *string          `json:"phoneNumber,omitempty"`
	BusinessScopedUserID       *string          `json:"businessScopedUserId,omitempty"`
	ParentBusinessScopedUserID *string          `json:"parentBusinessScopedUserId,omitempty"`
	WhatsAppUsername           *string          `json:"whatsappUsername,omitempty"`
	InstagramProfile           *InstagramProfile `json:"instagramProfile,omitempty"`
}

type InstagramProfile struct {
	IsFollower    *bool `json:"isFollower,omitempty"`
	IsFollowing   *bool `json:"isFollowing,omitempty"`
	FollowerCount *int  `json:"followerCount,omitempty"`
	IsVerified    *bool `json:"isVerified,omitempty"`
}

// InboxWebhookConversation is the conversation context in inbox webhook payloads.
type InboxWebhookConversation struct {
	ID                     string  `json:"id"`
	PlatformConversationID string  `json:"platformConversationId"`
	ParticipantID          *string `json:"participantId,omitempty"`
	ParticipantName        *string `json:"participantName,omitempty"`
	ParticipantUsername    *string `json:"participantUsername,omitempty"`
	ParticipantPicture     *string `json:"participantPicture,omitempty"`
	Status                 string  `json:"status"`
	ContactID              *string `json:"contactId,omitempty"`
}

// InboxWebhookAccount is the account context in inbox webhook payloads.
type InboxWebhookAccount struct {
	ID          string  `json:"id"`
	AccountID   string  `json:"accountId"`
	ProfileID   *string `json:"profileId,omitempty"`
	Platform    string  `json:"platform"`
	Username    string  `json:"username"`
	DisplayName *string `json:"displayName,omitempty"`
}

// ─── Webhook Payloads ──────────────────────────────────────────

// WebhookPayload is the envelope for all Zernio webhook events.
type WebhookPayload struct {
	ID        string          `json:"id"`
	Event     string          `json:"event"`
	Timestamp time.Time       `json:"timestamp"`
	RawBody   json.RawMessage `json:"-"`
}

// WebhookMessageReceived is the full payload for message.received events.
type WebhookMessageReceived struct {
	ID           string                    `json:"id"`
	Event        string                    `json:"event"`
	Message      InboxWebhookMessage       `json:"message"`
	Conversation InboxWebhookConversation  `json:"conversation"`
	Account      InboxWebhookAccount       `json:"account"`
	Metadata     *WebhookMetadata          `json:"metadata,omitempty"`
	Timestamp    time.Time                 `json:"timestamp"`
}

// WebhookMetadata carries platform-specific message context.
type WebhookMetadata struct {
	QuotedMessageID  *string                `json:"quotedMessageId,omitempty"`
	QuickReplyPayload *string               `json:"quickReplyPayload,omitempty"`
	PostbackPayload  *string                `json:"postbackPayload,omitempty"`
	PostbackTitle    *string                `json:"postbackTitle,omitempty"`
	CallbackData     *string                `json:"callbackData,omitempty"`
	InteractiveType  *string                `json:"interactiveType,omitempty"`
	InteractiveID    *string                `json:"interactiveId,omitempty"`
	ButtonPayload    *string                `json:"buttonPayload,omitempty"`
	FlowResponseJSON *string                `json:"flowResponseJson,omitempty"`
	FlowResponseData map[string]interface{} `json:"flowResponseData,omitempty"`
	Referral         *WebhookReferral       `json:"referral,omitempty"`
	StoryReply       *StoryReply            `json:"storyReply,omitempty"`
	IsStoryMention   *bool                  `json:"isStoryMention,omitempty"`
}

type WebhookReferral struct {
	CtwaClid     string `json:"ctwa_clid"`
	SourceID     string `json:"source_id"`
	SourceType   string `json:"source_type"`
	SourceURL    string `json:"source_url"`
	Headline     string `json:"headline"`
	Body         string `json:"body"`
	MediaType    string `json:"media_type"`
	ImageURL     string `json:"image_url"`
	VideoURL     string `json:"video_url"`
	ThumbnailURL string `json:"thumbnail_url"`
	AdID         string `json:"ad_id"`
	Ref          string `json:"ref"`
}

type StoryReply struct {
	StoryID  string  `json:"storyId"`
	StoryURL *string `json:"storyUrl,omitempty"`
}

// WebhookConversationStarted is the full payload for conversation.started events.
type WebhookConversationStarted struct {
	ID           string                   `json:"id"`
	Event        string                   `json:"event"`
	Conversation InboxWebhookConversation `json:"conversation"`
	Account      InboxWebhookAccount      `json:"account"`
	StartedAt    time.Time                `json:"startedAt"`
	Timestamp    time.Time                `json:"timestamp"`
}

// ─── API Response Types ────────────────────────────────────────

// ListConversationsResponse is the response from GET /v1/inbox/conversations.
type ListConversationsResponse struct {
	Data       []ConversationData `json:"data"`
	Pagination Pagination         `json:"pagination"`
	Meta       AccountsMeta       `json:"meta"`
}

type ConversationData struct {
	ID                   string            `json:"id"`
	Platform             string            `json:"platform"`
	AccountID            string            `json:"accountId"`
	AccountUsername       string            `json:"accountUsername"`
	ParticipantID        string            `json:"participantId"`
	ParticipantName      string            `json:"participantName"`
	ParticipantPicture   string            `json:"participantPicture"`
	LastMessage          string            `json:"lastMessage"`
	UpdatedTime          time.Time         `json:"updatedTime"`
	Status               string            `json:"status"`
	UnreadCount          int               `json:"unreadCount"`
	URL                  *string           `json:"url,omitempty"`
	InstagramProfile     *InstagramProfile `json:"instagramProfile,omitempty"`
}

type Pagination struct {
	HasMore    bool    `json:"hasMore"`
	NextCursor *string `json:"nextCursor,omitempty"`
}

type AccountsMeta struct {
	AccountsQueried int            `json:"accountsQueried"`
	AccountsFailed  int            `json:"accountsFailed"`
	FailedAccounts  []FailedAccount `json:"failedAccounts,omitempty"`
	LastUpdated     time.Time      `json:"lastUpdated"`
}

type FailedAccount struct {
	AccountID       string `json:"accountId"`
	AccountUsername string `json:"accountUsername"`
	Platform        string `json:"platform"`
	Error           string `json:"error"`
	Code            string `json:"code"`
	RetryAfter      int    `json:"retryAfter"`
}

// SendMessageRequest is the body for POST /v1/inbox/conversations/{id}/messages.
type SendMessageRequest struct {
	AccountID      string           `json:"accountId"`
	Message        *string          `json:"message,omitempty"`
	AttachmentURL  *string          `json:"attachmentUrl,omitempty"`
	AttachmentType *string          `json:"attachmentType,omitempty"`
	ReplyTo        *string          `json:"replyTo,omitempty"`
	Template       *TemplatePayload `json:"template,omitempty"`
	QuickReplies   []QuickReply     `json:"quickReplies,omitempty"`
	Buttons        []ActionButton   `json:"buttons,omitempty"`
}

type TemplatePayload struct {
	Type     string            `json:"type"`
	Elements []TemplateElement `json:"elements"`
}

type TemplateElement struct {
	Title    string         `json:"title"`
	Subtitle *string        `json:"subtitle,omitempty"`
	ImageURL *string        `json:"imageUrl,omitempty"`
	Buttons  []ActionButton `json:"buttons,omitempty"`
}

type QuickReply struct {
	Title    string `json:"title"`
	Payload  string `json:"payload"`
	ImageURL string `json:"imageUrl,omitempty"`
}

type ActionButton struct {
	Type    string `json:"type"`
	Title   string `json:"title"`
	URL     string `json:"url,omitempty"`
	Payload string `json:"payload,omitempty"`
	Phone   string `json:"phone,omitempty"`
}

// SendMessageResponse is the response from POST /v1/inbox/conversations/{id}/messages.
type SendMessageResponse struct {
	Success bool `json:"success"`
	Data    struct {
		MessageID      string             `json:"messageId"`
		ConversationID string             `json:"conversationId"`
		Attachments    []MessageAttachment `json:"attachments"`
	} `json:"data"`
}

// ListMessagesResponse is the response from GET /v1/inbox/conversations/{id}/messages.
type ListMessagesResponse struct {
	Data       []InboxWebhookMessage `json:"data"`
	Pagination Pagination            `json:"pagination"`
}

// ContactData represents a Zernio CRM contact.
type ContactData struct {
	ID                    string                 `json:"id"`
	Name                  *string                `json:"name,omitempty"`
	Email                 *string                `json:"email,omitempty"`
	Company               *string                `json:"company,omitempty"`
	AvatarURL             *string                `json:"avatarUrl,omitempty"`
	Tags                  []string               `json:"tags"`
	IsSubscribed          bool                   `json:"isSubscribed"`
	IsBlocked             bool                   `json:"isBlocked"`
	LastMessageSentAt     *time.Time             `json:"lastMessageSentAt,omitempty"`
	LastMessageReceivedAt *time.Time             `json:"lastMessageReceivedAt,omitempty"`
	MessagesSentCount     int                    `json:"messagesSentCount"`
	MessagesReceivedCount int                    `json:"messagesReceivedCount"`
	CustomFields          map[string]interface{} `json:"customFields"`
	Notes                 *string                `json:"notes,omitempty"`
	CreatedAt             time.Time              `json:"createdAt"`
	Platform              string                 `json:"platform"`
	PlatformIdentifier    string                 `json:"platformIdentifier"`
	DisplayIdentifier     string                 `json:"displayIdentifier"`
}

// AccountData represents a connected social account.
type AccountData struct {
	ID          string `json:"id"`
	Platform    string `json:"platform"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName,omitempty"`
	ProfileID   string `json:"profileId,omitempty"`
	Status      string `json:"status,omitempty"`
}
