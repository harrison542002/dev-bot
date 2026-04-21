package bot

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/bwmarrin/discordgo"

	"github.com/harrison542002/dev-bot/internal/config"
)

type discordPlatform struct {
	cfg       *config.DiscordConfig
	session   *discordgo.Session
	allowed   map[string]struct{}
	prefix    string
	onMessage func(ctx context.Context, sessionKey, text string, notify func(string))
}

func newDiscordPlatform(cfg *config.Config, onMessage func(context.Context, string, string, func(string))) (*discordPlatform, error) {
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
		onMessage: onMessage,
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
		p.send(ch.ID, msg)
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
	if text == "" {
		return
	}

	// Normalize command prefix so dispatch sees "/" uniformly.
	// Free-text messages (wizard replies) are passed through unchanged.
	if strings.HasPrefix(text, p.prefix) {
		text = "/" + strings.TrimPrefix(text, p.prefix)
	}

	channelID := m.ChannelID
	notify := func(reply string) {
		p.send(channelID, reply)
	}

	sessionKey := "dc:" + channelID + ":" + m.Author.ID
	ctx := context.Background()
	p.onMessage(ctx, sessionKey, text, notify)
}

func (p *discordPlatform) send(channelID, text string) {
	adapter := newChunkedMessageAdapter(
		1900, // leave headroom below Discord's 2000-char limit
		func(chunk string) error {
			_, err := p.session.ChannelMessageSend(channelID, chunk)
			return err
		},
		func(err error) {
			slog.Warn("discord: failed to send message", "channel_id", channelID, "err", err)
		},
	)
	adapter.Send(text)
}
