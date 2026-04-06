package bot

import (
	"context"
	"log/slog"
	"strings"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"devbot/internal/agent"
	"devbot/internal/config"
	ghclient "devbot/internal/github"
	"devbot/internal/task"
)

type Bot struct {
	cfg        *config.Config
	tg         *tgbot.Bot
	taskSvc    *task.Service
	gh         *ghclient.Client
	ag         *agent.Agent
	allowedIDs map[int64]struct{}
}

func New(cfg *config.Config, taskSvc *task.Service, gh *ghclient.Client, ag *agent.Agent) (*Bot, error) {
	allowed := make(map[int64]struct{}, len(cfg.Telegram.AllowedUserIDs))
	for _, id := range cfg.Telegram.AllowedUserIDs {
		allowed[id] = struct{}{}
	}

	b := &Bot{
		cfg:        cfg,
		taskSvc:    taskSvc,
		gh:         gh,
		ag:         ag,
		allowedIDs: allowed,
	}

	tg, err := tgbot.New(cfg.Telegram.Token, tgbot.WithDefaultHandler(b.handleMessage))
	if err != nil {
		return nil, err
	}
	b.tg = tg
	return b, nil
}

func (b *Bot) Start(ctx context.Context) {
	slog.Info("Telegram bot starting (polling)")
	b.tg.Start(ctx)
}

func (b *Bot) handleMessage(ctx context.Context, bot *tgbot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}
	msg := update.Message
	chatID := msg.Chat.ID
	userID := msg.From.ID

	// Auth: silently drop messages from non-allowlisted users
	if _, ok := b.allowedIDs[userID]; !ok {
		slog.Warn("dropping message from unknown user", "user_id", userID)
		return
	}

	text := strings.TrimSpace(msg.Text)
	if text == "" {
		return
	}

	// notify is a closure that sends a reply to this chat
	notify := func(reply string) {
		if _, err := bot.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   reply,
		}); err != nil {
			slog.Warn("failed to send message", "chat_id", chatID, "err", err)
		}
	}

	parts := strings.Fields(text)
	if len(parts) == 0 {
		return
	}

	switch parts[0] {
	case "/task":
		handleTask(ctx, b, chatID, parts[1:], notify)
	case "/pr":
		handlePR(ctx, b, chatID, parts[1:], notify)
	case "/status":
		handleStatus(ctx, b, notify)
	case "/help":
		handleHelp(notify)
	default:
		notify("Unknown command. Send /help to see available commands.")
	}
}

func (b *Bot) send(ctx context.Context, chatID int64, text string) {
	if _, err := b.tg.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID: chatID,
		Text:   text,
	}); err != nil {
		slog.Warn("send message failed", "chat_id", chatID, "err", err)
	}
}
