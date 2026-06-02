package tg

import (
	"encoding/json"
	"fmt"
)

type Response struct {
	OK          bool               `json:"ok"`
	Description string             `json:"description"`
	ErrorCode   int                `json:"error_code"`
	Result      json.RawMessage    `json:"result"`
	Parameters  ResponseParameters `json:"parameters"`
}

func DecodeResponse[T any](r *Response) (*T, error) {
	var result T
	if err := json.Unmarshal(r.Result, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

type ResponseParameters struct {
	MigrateToChatID ChatID `json:"migrate_to_chat_id"`
	RetryAfter      int    `json:"retry_after"`
}

type Update struct {
	UpdateID int      `json:"update_id"`
	Message  *Message `json:"message"`
}

func (u Update) String() string {
	return fmt.Sprintf("{UpdateID: %d, Message: %v}", u.UpdateID, u.Message)
}

type MessageID int

type Message struct {
	MessageID MessageID        `json:"message_id"`
	Text      string           `json:"text"`
	Chat      Chat             `json:"chat"`
	From      User             `json:"from"`
	Entities  *[]MessageEntity `json:"entities"`
}

func (m Message) String() string {
	return fmt.Sprintf(
		"{MessageID: %d, Text: %q, Chat: %v, From: %v}",
		m.MessageID, m.Text, m.Chat, m.From)
}

type MessageEntityType string

const (
	MessageEntityBotCommand = "bot_command"
)

type MessageEntity struct {
	Type   MessageEntityType `json:"type"`
	Offset int               `json:"offset"`
	Length int               `json:"length"`
}

type BotCommand struct {
	Command     string `json:"command"`
	Description string `json:"description"`
}

type BotName struct {
	Name string `json:"name"`
}

type User struct {
	ID       int64  `json:"id"`
	IsBot    bool   `json:"is_bot"`
	Username string `json:"username"`
}

type ChatID int64

type Chat struct {
	ID ChatID `json:"id"`
}

type ReplyParameters struct {
	MessageID MessageID `json:"message_id"`
}

type ReactionType struct {
	Type          string `json:"type"`
	Emoji         string `json:"emoji,omitempty"`
	CustomEmojiID string `json:"custom_emoji_id,omitempty"`
}

type ReactionEmoji string

const (
	ReactionEmojiClown = "🤡"
)

func NewReactionTypeEmoji(emoji ReactionEmoji) *ReactionType {
	return &ReactionType{
		Type:  "emoji",
		Emoji: string(emoji),
	}
}
