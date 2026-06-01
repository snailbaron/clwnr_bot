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

type MessageToAct struct {
	ChatID    tg.ChatID
	MessageID tg.MessageID
}

const (
	POLL_TIMEOUT = 15 * time.Second
	QUEUE_SIZE   = 1000
)

type Clowner struct {
	botUsername  string
	tgClient     *tg.Client
	chatSettings map[tg.ChatID]*Settings
	rand         *rand.Rand

	updateOffset int
	messages     chan *MessageToAct
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
		rand:         rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64())),
		messages:     make(chan *MessageToAct, QUEUE_SIZE),
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
	ClownerCommandP    ClownerCommandType = "p"
	ClownerCommandStop ClownerCommandType = "stop"
	ClownerCommandInfo ClownerCommandType = "info"
)

type ClownerCommand struct {
	MessageID tg.MessageID
	ChatID    tg.ChatID
	Command   ClownerCommandType
	Args      string
}

func (c *Clowner) parseBotCommand(msg *tg.Message) *ClownerCommand {
	if msg.Entities == nil {
		return nil
	}

	slog.Debug("parsing message entities", "n", len(*msg.Entities))
	for _, e := range *msg.Entities {
		if e.Offset != 0 {
			slog.Debug("entity offset is not zero", "offset", e.Offset)
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
			slog.Debug("command does not start with /", "command", commandAndName)
			continue
		}

		parts := strings.SplitN(commandAndName, "@", 2)
		if len(parts) != 2 {
			slog.Debug("command does not have @", "command", commandAndName)
			continue
		}

		command, name := parts[0], parts[1]
		if name != c.botUsername {
			slog.Debug("foreign bot name",
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
				"Мне нужен процент, а %q это не процент, а клоунада!",
				cmd.Args)
			break
		}

		percent = max(0, min(100, percent))
		sendf("Ставлю клоунение в %d%%!", percent)
		s.PercentClown = percent
	case ClownerCommandStop:
		sendf("Нет!")
	case ClownerCommandInfo:
		sendf("На этой арене заведено так:\n"+
			" * Клоунение %d%%\n"+
			" * %s",
			s.PercentClown, clownActionUserDescription(s.Action))
	default:
		sendf(
			"🎪%s🎪 это что-то новенькое! У меня в программе нет такого номера!",
			cmd.Command)
	}
}

func (c *Clowner) mustClown(s *Settings) bool {
	p := float64(s.PercentClown) / 100
	return c.rand.Float64() < p
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
	for {
		updates, err := c.tgClient.GetUpdatesMessages(
			c.updateOffset, POLL_TIMEOUT)
		if err != nil {
			slog.Error("failed to get updates:", "error", err)
			continue
		}

		for _, u := range updates {
			c.updateOffset = max(c.updateOffset, u.UpdateID+1)

			if u.Message == nil || u.Message.From.IsBot {
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
