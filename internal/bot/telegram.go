package bot

import (
	"context"
	"log/slog"
	"strconv"
	"strings"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/harrison542002/dev-bot/internal/config"
)

type telegramPlatform struct {
	cfg       *config.TelegramConfig
	tg        *tgbot.Bot
	allowed   map[int64]struct{}
	onMessage func(ctx context.Context, sessionKey, text string, notify func(string))
}

func newTelegramPlatform(cfg *config.Config, onMessage func(context.Context, string, string, func(string))) (*telegramPlatform, error) {
	allowed := make(map[int64]struct{}, len(cfg.Telegram.AllowedUserIDs))
	for _, id := range cfg.Telegram.AllowedUserIDs {
		allowed[id] = struct{}{}
	}
	p := &telegramPlatform{
		cfg:       &cfg.Telegram,
		allowed:   allowed,
		onMessage: onMessage,
	}
	tg, err := tgbot.New(cfg.Telegram.Token, tgbot.WithDefaultHandler(p.handleMessage))
	if err != nil {
		return nil, err
	}
	p.tg = tg
	return p, nil
}

func (p *telegramPlatform) Start(ctx context.Context) {
	slog.Info("Telegram bot starting (polling)")
	p.tg.Start(ctx)
}

// BroadcastMessage sends msg to every allowlisted user.
// In Telegram, direct-message chatID == userID, so allowed_user_ids doubles as chatIDs.
func (p *telegramPlatform) BroadcastMessage(msg string) {
	ctx := context.Background()
	for _, userID := range p.cfg.AllowedUserIDs {
		p.send(ctx, userID, msg)
	}
}

func (p *telegramPlatform) handleMessage(ctx context.Context, _ *tgbot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}
	msg := update.Message
	chatID := msg.Chat.ID
	userID := msg.From.ID

	// Auth: silently drop messages from non-allowlisted users
	if _, ok := p.allowed[userID]; !ok {
		slog.Warn("telegram: dropping message from unknown user", "user_id", userID)
		return
	}

	text := strings.TrimSpace(msg.Text)
	if text == "" {
		return
	}

	notify := func(reply string) {
		println("Reply : ", reply)
		p.sendChunked(ctx, chatID, reply)
	}

	sessionKey := "tg:" + strconv.FormatInt(chatID, 10)
	p.onMessage(ctx, sessionKey, text, notify)
}

func (p *telegramPlatform) send(ctx context.Context, chatID int64, text string) {
	p.sendChunked(ctx, chatID, text)
}

// sendChunked splits long messages to respect Telegram's 4096-character limit.
func (p *telegramPlatform) sendChunked(ctx context.Context, chatID int64, text string) {
	const maxLen = 4000 // leave headroom below Telegram's 4096-char limit
	for _, chunk := range splitMessage(text, maxLen) {
		if _, err := p.tg.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   chunk,
		}); err != nil {
			slog.Warn("telegram: send message failed", "chat_id", chatID, "err", err)
			return
		}
	}
}

func splitMessage(text string, maxLen int) []string {
	if text == "" || maxLen <= 0 {
		return nil
	}

	runes := []rune(text)
	chunks := make([]string, 0, len(runes)/maxLen+1)
	for len(runes) > maxLen {
		splitAt := maxLen
		for i := maxLen - 1; i >= maxLen/2; i-- {
			if runes[i] == '\n' {
				splitAt = i + 1
				break
			}
		}
		chunks = append(chunks, string(runes[:splitAt]))
		runes = runes[splitAt:]
	}
	if len(runes) > 0 {
		chunks = append(chunks, string(runes))
	}
	return chunks
}
