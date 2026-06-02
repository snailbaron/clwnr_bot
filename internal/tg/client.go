package tg

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"
)

const (
	CLIENT_TIMEOUT      = 30 * time.Second
	RETRY_COUNT         = 3
	RETRY_DEFAULT_DELAY = 5 * time.Second
)

type Client struct {
	client  *http.Client
	baseURL string
}

func (c *Client) tryPostJSONBytes(
	endpoint string, jsonBytes []byte) (*http.Response, *Response, error) {

	r, err := c.client.Post(
		c.baseURL+"/"+endpoint,
		"application/json",
		bytes.NewReader(jsonBytes))
	if err != nil {
		return nil, nil, err
	}
	defer r.Body.Close()

	respBytes, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, nil, err
	}

	var respStruct Response
	if err := json.Unmarshal(respBytes, &respStruct); err != nil {
		return nil, nil, err
	}

	return r, &respStruct, nil
}

func (c *Client) postJSON(
	endpoint string, request map[string]any) (*Response, error) {

	jsonBytes, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	slog.Debug("sending request:", "endpoint", endpoint, "json", string(jsonBytes))

	for retry := 0; ; retry++ {
		resp, respStruct, err := c.tryPostJSONBytes(endpoint, jsonBytes)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode == 429 {
			delay := RETRY_DEFAULT_DELAY
			if sec := respStruct.Parameters.RetryAfter; sec > 0 {
				delay = time.Duration(sec) * time.Second
			}

			if retry < RETRY_COUNT {
				slog.Warn(
					"got 429 response, sleeping:",
					"description", respStruct.Description, "delay", delay)
				time.Sleep(delay)
				continue
			} else {
				return nil, fmt.Errorf(
					"request returned 429 after %d retries", RETRY_COUNT)
			}
		}

		if resp.StatusCode/100 != 2 {
			return nil, fmt.Errorf("request failed: %s", resp.Status)
		}

		if !respStruct.OK {
			return nil, fmt.Errorf(
				"request failed (error code %d): %s",
				respStruct.ErrorCode, respStruct.Description)
		}

		return respStruct, nil
	}
}

func (c *Client) post(
	endpoint string) (*Response, error) {

	return c.postJSON(endpoint, map[string]any{})
}

func NewClient(token string) *Client {
	return &Client{
		client: &http.Client{Timeout: CLIENT_TIMEOUT},
		baseURL: (&url.URL{
			Scheme: "https",
			Host:   "api.telegram.org",
			Path:   fmt.Sprintf("bot%s", token),
		}).String(),
	}
}

func (c *Client) GetMe() (*User, error) {
	r, err := c.post("getMe")
	if err != nil {
		return nil, err
	}

	user, err := DecodeResponse[User](r)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (c *Client) GetMyName() (string, error) {
	r, err := c.postJSON("getMyName", map[string]any{})
	if err != nil {
		return "", err
	}

	botName, err := DecodeResponse[BotName](r)
	if err != nil {
		return "", err
	}

	return botName.Name, nil
}

func (c *Client) SetMyCommands(cmds []BotCommand) error {
	_, err := c.postJSON("setMyCommands", map[string]any{
		"commands": cmds,
	})
	return err
}

func (c *Client) GetUpdatesMessages(
	offset int, pollTimeout time.Duration) ([]Update, error) {

	r, err := c.postJSON("getUpdates", map[string]any{
		"offset":          offset,
		"timeout":         int(pollTimeout.Seconds()),
		"allowed_updates": []string{"message"},
	})
	if err != nil {
		return nil, err
	}

	updates, err := DecodeResponse[[]Update](r)
	if err != nil {
		return nil, err
	}

	return *updates, nil
}

func (c *Client) SetMessageReaction(
	chatID ChatID, messageID MessageID, emoji ReactionEmoji) error {

	_, err := c.postJSON("setMessageReaction", map[string]any{
		"chat_id":    chatID,
		"message_id": messageID,
		"reaction":   []*ReactionType{NewReactionTypeEmoji(emoji)},
	})
	return err
}

func (c *Client) SendMessageReply(
	chatID ChatID, text string, messageID MessageID) error {

	_, err := c.postJSON("sendMessage", map[string]any{
		"chat_id":          chatID,
		"text":             text,
		"reply_parameters": ReplyParameters{MessageID: messageID},
	})
	return err
}
