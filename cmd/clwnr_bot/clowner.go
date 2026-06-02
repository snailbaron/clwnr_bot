package main

import (
	"fmt"
	"log/slog"
	"math/rand/v2"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/snailbaron/clwnr_bot/internal/tg"
)

type ClownActionType int

const (
	ClownActionReaction ClownActionType = iota
	ClownActionResponse
)

func clownActionUserDescription(clownAction ClownActionType) string {
	switch clownAction {
	case ClownActionReaction:
		return "При случае ставим реакцию"
	case ClownActionResponse:
		return "При случае шлём ответное сообщение"
	default:
		return "Случилось что-то странное, и на сообщения мы не реагируем..."
	}
}

// Settings contain Clowner settings for a chat.
type Settings struct {
	// PercentClown is the probability (in percents) to clown another message,
	// indiscriminately applied to each of the incoming messages.
	PercentClown int

	// Action describes the action taken when clowning a message.
	// ClownActionReaction sets a clown reaction. ClownActionResponse responds
	// with a clown emoji.
	Action ClownActionType
}

func NewSettings() *Settings {
	return &Settings{
		PercentClown: 1,
		Action:       ClownActionResponse,
	}
}

const (
	POLL_TIMEOUT           = 15 * time.Second
	POLL_RETRY_START_DELAY = 3 * time.Second
	POLL_RETRY_MAX_DELAY   = 5 * time.Minute
)

type Clowner struct {
	botUsername  string
	tgClient     *tg.Client
	chatSettings map[tg.ChatID]*Settings
	updateOffset int
}

func NewClowner(token string) (*Clowner, error) {
	tgClient := tg.NewClient(token)

	botUser, err := tgClient.GetMe()
	if err != nil {
		return nil, err
	}

	return &Clowner{
		botUsername:  botUser.Username,
		tgClient:     tgClient,
		chatSettings: make(map[tg.ChatID]*Settings),
	}, nil
}

func (c *Clowner) getChatSettings(chatID tg.ChatID) *Settings {
	s, ok := c.chatSettings[chatID]
	if !ok {
		s = NewSettings()
		c.chatSettings[chatID] = s
	}
	return s
}

type ClownerCommandType string

const (
	ClownerCommandP       ClownerCommandType = "p"
	ClownerCommandStop    ClownerCommandType = "stop"
	ClownerCommandInfo    ClownerCommandType = "info"
	ClownerCommandReact   ClownerCommandType = "react"
	ClownerCommandRespond ClownerCommandType = "respond"
)

var botCommands = []tg.BotCommand{
	{
		Command:     string(ClownerCommandP),
		Description: "Установить уровень клоунения в процентах",
	},
	{
		Command:     string(ClownerCommandStop),
		Description: "Остановить Клоунщика",
	},
	{
		Command:     string(ClownerCommandInfo),
		Description: "Показать настройки чата",
	},
	{
		Command:     string(ClownerCommandReact),
		Description: "Ставить реакцию-клоуна",
	},
	{
		Command:     string(ClownerCommandRespond),
		Description: "Отвечать сообщениями с клоуном",
	},
}

type ClownerCommand struct {
	MessageID tg.MessageID
	ChatID    tg.ChatID
	Command   ClownerCommandType
	Args      string
}

func (c *Clowner) parseBotCommand(msg *tg.Message) *ClownerCommand {
	slog.Debug("parsing message entities:", "n", len(msg.Entities))
	for _, e := range msg.Entities {
		if e.Type != tg.MessageEntityBotCommand {
			slog.Debug("entity type is not bot_command:", "type", e.Type)
			continue
		}

		if e.Offset != 0 {
			slog.Debug("entity offset is not zero:", "offset", e.Offset)
			continue
		}

		spaceIndex := strings.IndexFunc(msg.Text, unicode.IsSpace)
		if spaceIndex == -1 {
			spaceIndex = len(msg.Text)
		}

		commandAndName := msg.Text[:spaceIndex]
		args := strings.TrimSpace(msg.Text[spaceIndex:])

		commandAndName, found := strings.CutPrefix(commandAndName, "/")
		if !found {
			slog.Debug("command does not start with /:", "command", commandAndName)
			continue
		}

		parts := strings.SplitN(commandAndName, "@", 2)
		if len(parts) != 2 {
			slog.Debug("command does not have @:", "command", commandAndName)
			continue
		}

		command, name := parts[0], parts[1]
		if name != c.botUsername {
			slog.Debug("foreign bot name:",
				"command", commandAndName,
				"bot_name", name,
				"my_name", c.botUsername)
			continue
		}

		return &ClownerCommand{
			MessageID: msg.MessageID,
			ChatID:    msg.Chat.ID,
			Command:   ClownerCommandType(command),
			Args:      args,
		}
	}

	return nil
}

func (c *Clowner) processCommand(cmd *ClownerCommand, s *Settings) {
	sendf := func(format string, a ...any) {
		c.tgClient.SendMessageReply(
			cmd.ChatID,
			fmt.Sprintf(format, a...),
			cmd.MessageID)
	}

	switch cmd.Command {
	case ClownerCommandP:
		percent, err := strconv.Atoi(cmd.Args)
		if err != nil {
			sendf(
				"Мне нужно целое число, а 🎪%s🎪 это клоунада!",
				cmd.Args)
			break
		}

		percent = max(1, min(100, percent))
		sendf("Ставлю клоунение в %d%%!", percent)
		s.PercentClown = percent
	case ClownerCommandStop:
		sendf("Нет!")
	case ClownerCommandInfo:
		sendf("На этой арене заведено так:\n"+
			" * Клоунение %d%%\n"+
			" * %s",
			s.PercentClown, clownActionUserDescription(s.Action))
	case ClownerCommandReact:
		sendf("Будем реагировать!")
		s.Action = ClownActionReaction
	case ClownerCommandRespond:
		sendf("Будем отвечать!")
		s.Action = ClownActionResponse
	default:
		sendf(
			"🎪%s🎪 это что-то новенькое! У меня в программе нет такого номера!",
			cmd.Command)
	}
}

func (c *Clowner) mustClown(s *Settings) bool {
	p := float64(s.PercentClown) / 100
	return rand.Float64() < p
}

func (c *Clowner) actToMessage(m *tg.Message, s *Settings) {
	var err error
	switch s.Action {
	case ClownActionReaction:
		err = c.tgClient.SetMessageReaction(
			m.Chat.ID, m.MessageID, tg.ReactionEmojiClown)
	case ClownActionResponse:
		err = c.tgClient.SendMessageReply(m.Chat.ID, "🤡", m.MessageID)
	default:
		err = fmt.Errorf("unknown action %d", s.Action)
	}

	if err != nil {
		slog.Error("cannot react to message:", "error", err)
	}
}

func (c *Clowner) Run() {
	if err := c.tgClient.SetMyCommands(botCommands); err != nil {
		slog.Error("failed to set my commands:", "error", err)
	}

	for {
		var updates []tg.Update
		delay := POLL_RETRY_START_DELAY
		for {
			var err error
			updates, err = c.tgClient.GetUpdatesMessages(
				c.updateOffset, POLL_TIMEOUT)
			if err == nil {
				break
			}

			slog.Error(
				"failed to get updates, will retry:",
				"error", err, "delay", delay)
			time.Sleep(delay)
			delay = min(POLL_RETRY_MAX_DELAY, delay+delay)
		}

		for _, u := range updates {
			c.updateOffset = max(c.updateOffset, u.UpdateID+1)

			if u.Message == nil || u.Message.From != nil && u.Message.From.IsBot {
				continue
			}

			s := c.getChatSettings(u.Message.Chat.ID)
			if cmd := c.parseBotCommand(u.Message); cmd != nil {
				c.processCommand(cmd, s)
				continue
			}

			if c.mustClown(s) {
				c.actToMessage(u.Message, s)
			}
		}
	}
}
