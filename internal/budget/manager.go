// Package budget provides spend tracking and automatic provider fallback.
//
// When a monthly limit is configured, every successful API call's token usage
// is recorded. Once the limit is exceeded, the Manager switches to the local
// fallback provider transparently. The user can override this at any time with
// /budget pause (use commercial regardless) or /budget resume.
package budget

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/harrison542002/dev-bot/internal/llm"
	"github.com/harrison542002/dev-bot/internal/store"
)

// pricing stores approximate USD cost per million tokens for known models.
// Entries are matched by prefix (longest prefix wins).
// Prices are approximate and may drift; users can ignore cost tracking if they
// set budget.monthly_limit_usd: 0.
var pricingTable = []struct {
	prefix    string
	inputCPM  float64 // USD per million input tokens
	outputCPM float64 // USD per million output tokens
}{
	// Anthropic Claude
	{"claude-opus", 15.00, 75.00},
	{"claude-sonnet", 3.00, 15.00},
	{"claude-haiku", 0.80, 4.00},
	// OpenAI
	{"o3", 10.00, 40.00},
	{"gpt-4o", 2.50, 10.00},
	{"gpt-4-turbo", 10.00, 30.00},
	{"gpt-4", 30.00, 60.00},
	{"gpt-3.5", 0.50, 1.50},
	// Google Gemini
	{"gemini-2.0-flash", 0.10, 0.40},
	{"gemini-1.5-flash", 0.075, 0.30},
	{"gemini-1.5-pro", 1.25, 5.00},
	{"gemini-pro", 1.25, 5.00},
}

// fallbackCPM is used when no pricing entry matches (conservative estimate).
const fallbackInputCPM = 10.00
const fallbackOutputCPM = 30.00

// estimateCost returns the USD cost of the given usage for providerName.
// providerName is matched case-insensitively against pricingTable prefixes.
func estimateCost(providerName string, usage *llm.Usage) float64 {
	if usage == nil {
		return 0
	}
	name := strings.ToLower(providerName)

	// Strip "Local (" wrapper if present
	if strings.HasPrefix(name, "local (") {
		name = strings.TrimPrefix(name, "local (")
		name = strings.TrimSuffix(name, ")")
	}

	// Longest-prefix match
	bestLen := 0
	inputCPM := fallbackInputCPM
	outputCPM := fallbackOutputCPM

	for _, p := range pricingTable {
		if strings.HasPrefix(name, p.prefix) && len(p.prefix) > bestLen {
			bestLen = len(p.prefix)
			inputCPM = p.inputCPM
			outputCPM = p.outputCPM
		}
	}

	return float64(usage.InputTokens)/1e6*inputCPM +
		float64(usage.OutputTokens)/1e6*outputCPM
}

// currentMonth returns the current month key in "2006-01" format (UTC).
func currentMonth() string {
	return time.Now().UTC().Format("2006-01")
}

// Manager implements llm.Client with budget-aware provider selection.
// It wraps a primary (commercial) client and an optional local fallback.
// When the monthly spend exceeds the configured limit, the local client is used.
type Manager struct {
	primary   llm.Client // commercial provider
	fallback  llm.Client // local provider; nil = no fallback configured
	store     store.Store
	limitUSD  float64 // 0 = unlimited

	mu        sync.Mutex
	paused    bool   // when true, skip budget check and always use primary
	notified  string // month key of last "budget exceeded" broadcast
	broadcast func(string)
}

// New creates a Manager. fallback may be nil (disabled).
// broadcast is called when the provider switches automatically; may be nil.
func New(primary, fallback llm.Client, s store.Store, limitUSD float64, broadcast func(string)) *Manager {
	return &Manager{
		primary:   primary,
		fallback:  fallback,
		store:     s,
		limitUSD:  limitUSD,
		broadcast: broadcast,
	}
}

// SetBroadcast injects the broadcast function after construction (breaks init cycle).
func (m *Manager) SetBroadcast(fn func(string)) {
	m.mu.Lock()
	m.broadcast = fn
	m.mu.Unlock()
}

// ProviderName returns the currently active provider's name.
func (m *Manager) ProviderName() string {
	active, _ := m.activeClient(context.Background())
	return active.ProviderName()
}

// Complete selects the appropriate provider, calls it, and records the usage.
func (m *Manager) Complete(ctx context.Context, system, user string, maxTokens int) (string, *llm.Usage, error) {
	active, switched := m.activeClient(ctx)

	// If we just switched to fallback for the first time this month, notify.
	if switched {
		m.maybeNotifySwitch(ctx, active)
	}

	text, usage, err := active.Complete(ctx, system, user, maxTokens)
	if err != nil {
		return "", nil, err
	}

	// Record usage asynchronously to not block the response.
	if usage != nil {
		cost := estimateCost(active.ProviderName(), usage)
		go func() {
			if rerr := m.store.AddBudgetUsage(
				context.Background(),
				currentMonth(),
				active.ProviderName(),
				usage.InputTokens,
				usage.OutputTokens,
				cost,
			); rerr != nil {
				slog.Warn("failed to record budget usage", "err", rerr)
			}
		}()
	}

	return text, usage, nil
}

// CompleteWithTools implements llm.ToolUser, delegating to the active provider
// if it supports tool use. Returns an error if the active provider does not
// support tools — this happens when budget switches mid-task to a local model
// that lacks tool-use capability. The task will reset to TODO and be retried
// on the next scheduler tick using the single-shot path.
func (m *Manager) CompleteWithTools(ctx context.Context, system string, messages []llm.Message, tools []llm.Tool, maxTokens int) (llm.Message, *llm.Usage, error) {
	active, switched := m.activeClient(ctx)
	if switched {
		m.maybeNotifySwitch(ctx, active)
	}

	tu, ok := active.(llm.ToolUser)
	if !ok {
		return llm.Message{}, nil, fmt.Errorf(
			"active provider %s does not support tool use (budget may have switched to local); task will be retried",
			active.ProviderName(),
		)
	}

	msg, usage, err := tu.CompleteWithTools(ctx, system, messages, tools, maxTokens)
	if err != nil {
		return llm.Message{}, nil, err
	}

	if usage != nil {
		cost := estimateCost(active.ProviderName(), usage)
		go func() {
			if rerr := m.store.AddBudgetUsage(
				context.Background(),
				currentMonth(),
				active.ProviderName(),
				usage.InputTokens,
				usage.OutputTokens,
				cost,
			); rerr != nil {
				slog.Warn("failed to record budget usage", "err", rerr)
			}
		}()
	}

	return msg, usage, nil
}

// activeClient returns the provider to use right now plus a bool indicating
// whether it just switched from primary to fallback for the first time.
func (m *Manager) activeClient(ctx context.Context) (llm.Client, bool) {
	m.mu.Lock()
	paused := m.paused
	notified := m.notified
	m.mu.Unlock()

	if m.limitUSD <= 0 || paused || m.fallback == nil {
		return m.primary, false
	}

	spent, err := m.store.GetMonthlySpend(ctx, currentMonth())
	if err != nil {
		slog.Warn("budget check failed, using primary", "err", err)
		return m.primary, false
	}

	if spent < m.limitUSD {
		return m.primary, false
	}

	// Over budget — use fallback
	switched := notified != currentMonth()
	return m.fallback, switched
}

func (m *Manager) maybeNotifySwitch(ctx context.Context, active llm.Client) {
	m.mu.Lock()
	if m.notified == currentMonth() {
		m.mu.Unlock()
		return
	}
	m.notified = currentMonth()
	broadcast := m.broadcast
	m.mu.Unlock()

	if broadcast != nil {
		spent, _ := m.store.GetMonthlySpend(ctx, currentMonth())
		msg := fmt.Sprintf(
			"Budget limit of $%.2f reached (spent $%.2f this month).\nSwitching to %s for the rest of the month.\nUse /budget pause to override, or /budget resume to re-enable automatic switching.",
			m.limitUSD, spent, active.ProviderName(),
		)
		broadcast(msg)
	}
}

// Pause disables budget enforcement — always uses the primary (commercial) provider.
func (m *Manager) Pause() {
	m.mu.Lock()
	m.paused = true
	m.mu.Unlock()
}

// Resume re-enables budget enforcement.
func (m *Manager) Resume() {
	m.mu.Lock()
	m.paused = false
	m.mu.Unlock()
}

// IsPaused reports whether budget enforcement is currently paused.
func (m *Manager) IsPaused() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.paused
}

// HasFallback reports whether a local fallback provider is configured.
func (m *Manager) HasFallback() bool {
	return m.fallback != nil
}

// LimitUSD returns the configured monthly limit (0 = unlimited).
func (m *Manager) LimitUSD() float64 {
	return m.limitUSD
}

// Status returns a formatted status string suitable for a Telegram message.
func (m *Manager) Status(ctx context.Context) string {
	month := currentMonth()
	spent, _ := m.store.GetMonthlySpend(ctx, month)
	breakdown, _ := m.store.GetMonthlyBreakdown(ctx, month)

	var sb strings.Builder
	sb.WriteString("Budget — " + month + "\n\n")

	if m.limitUSD > 0 {
		remaining := m.limitUSD - spent
		if remaining < 0 {
			remaining = 0
		}
		pct := spent / m.limitUSD * 100
		sb.WriteString(fmt.Sprintf("Spent:     $%.4f / $%.2f (%.1f%%)\n", spent, m.limitUSD, pct))
		sb.WriteString(fmt.Sprintf("Remaining: $%.4f\n", remaining))
	} else {
		sb.WriteString(fmt.Sprintf("Spent:     $%.4f (no limit set)\n", spent))
	}

	if m.paused {
		sb.WriteString("Enforcement: PAUSED (always uses primary)\n")
	} else if m.fallback == nil {
		sb.WriteString("Enforcement: no local fallback configured\n")
	} else if m.limitUSD <= 0 {
		sb.WriteString("Enforcement: disabled (no limit)\n")
	} else if spent >= m.limitUSD {
		sb.WriteString(fmt.Sprintf("Enforcement: ACTIVE — using %s\n", m.fallback.ProviderName()))
	} else {
		sb.WriteString(fmt.Sprintf("Enforcement: active — using %s\n", m.primary.ProviderName()))
	}

	if len(breakdown) > 0 {
		sb.WriteString("\nBreakdown:\n")
		for _, r := range breakdown {
			sb.WriteString(fmt.Sprintf("  %-14s $%.4f  (%dk in / %dk out)\n",
				r.Provider,
				r.CostUSD,
				r.InputTokens/1000,
				r.OutputTokens/1000,
			))
		}
	}

	return sb.String()
}
