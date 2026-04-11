package bot

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/bwmarrin/discordgo"

	"devbot/internal/config"
)

type discordPlatform struct {
	cfg       *config.DiscordConfig
	session   *discordgo.Session
	allowed   map[string]struct{}
	prefix    string
	onCommand func(ctx context.Context, parts []string, notify func(string))
}

func newDiscordPlatform(cfg *config.Config, onCommand func(context.Context, []string, func(string))) (*discordPlatform, error) {
	allowed := make(map[string]struct{}, len(cfg.Discord.AllowedUserIDs))
	for _, id := range cfg.Discord.AllowedUserIDs {
		allowed[id] = struct{}{}
	}

	prefix := cfg.Discord.CommandPrefix
	if prefix == "" {
		prefix = "!"
	}

	dg, err := discordgo.New("Bot " + cfg.Discord.Token)
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}
	// Message Content Intent must be enabled in the Discord Developer Portal
	// under Bot → Privileged Gateway Intents, otherwise message text is empty.
	dg.Identify.Intents = discordgo.IntentsGuildMessages |
		discordgo.IntentsDirectMessages |
		discordgo.IntentMessageContent

	p := &discordPlatform{
		cfg:       &cfg.Discord,
		session:   dg,
		allowed:   allowed,
		prefix:    prefix,
		onCommand: onCommand,
	}
	dg.AddHandler(p.handleMessage)
	return p, nil
}

func (p *discordPlatform) Start(ctx context.Context) {
	if err := p.session.Open(); err != nil {
		slog.Error("discord: failed to open WebSocket connection", "err", err)
		return
	}
	slog.Info("Discord bot connected", "prefix", p.prefix)
	<-ctx.Done()
	slog.Info("Discord bot disconnecting")
	_ = p.session.Close()
}

// BroadcastMessage sends a DM to every allowlisted Discord user.
func (p *discordPlatform) BroadcastMessage(msg string) {
	for _, userID := range p.cfg.AllowedUserIDs {
		ch, err := p.session.UserChannelCreate(userID)
		if err != nil {
			slog.Warn("discord: failed to open DM channel", "user_id", userID, "err", err)
			continue
		}
		p.sendChunked(ch.ID, msg)
	}
}

func (p *discordPlatform) handleMessage(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author == nil || m.Author.ID == s.State.User.ID {
		return
	}
	if _, ok := p.allowed[m.Author.ID]; !ok {
		slog.Warn("discord: dropping message from unknown user", "user_id", m.Author.ID)
		return
	}

	text := strings.TrimSpace(m.Content)
	if !strings.HasPrefix(text, p.prefix) {
		return
	}

	text = strings.TrimPrefix(text, p.prefix)
	parts := strings.Fields(text)
	if len(parts) == 0 {
		return
	}
	// Prepend "/" so dispatch() routing matches Telegram convention.
	// e.g. "!task add foo" → ["/task", "add", "foo"]
	parts[0] = "/" + parts[0]

	channelID := m.ChannelID
	notify := func(reply string) {
		p.sendChunked(channelID, reply)
	}

	ctx := context.Background()
	p.onCommand(ctx, parts, notify)
}

// sendChunked splits long messages to respect Discord's 2000-character limit.
func (p *discordPlatform) sendChunked(channelID, text string) {
	const maxLen = 1900 // leave headroom below Discord's 2000-char limit
	for len(text) > maxLen {
		chunk := text[:maxLen]
		// Try to break on a newline to avoid splitting mid-line
		if idx := strings.LastIndex(chunk, "\n"); idx > maxLen/2 {
			chunk = text[:idx+1]
		}
		if _, err := p.session.ChannelMessageSend(channelID, chunk); err != nil {
			slog.Warn("discord: failed to send message chunk", "channel_id", channelID, "err", err)
			return
		}
		text = text[len(chunk):]
	}
	if len(text) > 0 {
		if _, err := p.session.ChannelMessageSend(channelID, text); err != nil {
			slog.Warn("discord: failed to send message", "channel_id", channelID, "err", err)
		}
	}
}
