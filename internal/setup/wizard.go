package setup

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Wizard struct {
	r *bufio.Reader
}

type repoEntry struct {
	owner      string
	repo       string
	name       string
	baseBranch string
}

type answers struct {
	platform string

	// telegram
	tgToken   string
	tgUserIDs []string

	// discord
	dsToken   string
	dsUserIDs []string
	dsPrefix  string

	// git identity
	gitName  string
	gitEmail string

	// github
	ghToken string
	repos   []repoEntry

	// ai
	aiProvider  string
	claudeKey   string
	claudeModel string
	openaiKey   string
	openaiModel string
	geminiKey   string
	geminiModel string
	localURL    string
	localModel  string
	codexModel  string
}

// Run displays the first-run Wizard and writes the resulting config to cfgPath.
func Run(cfgPath string) error {
	w := &Wizard{r: bufio.NewReader(os.Stdin)}
	return w.run(cfgPath)
}

func (w *Wizard) run(cfgPath string) error {
	w.header()
	fmt.Printf("No config found at %q. Let's create one.\n\n", cfgPath)

	a := &answers{}

	steps := []struct {
		label string
		fn    func(*answers) error
	}{
		{"Messaging Platform", w.collectPlatform},
		{"Git Identity", w.collectGitIdentity},
		{"GitHub", w.collectGitHub},
		{"AI Provider", w.collectAI},
	}

	for i, s := range steps {
		w.section(fmt.Sprintf("[%d/%d] %s", i+1, len(steps), s.label))
		if err := s.fn(a); err != nil {
			return err
		}
		fmt.Println()
	}

	if err := w.writeConfig(cfgPath, a); err != nil {
		return err
	}
	fmt.Printf("\nConfig written to %q. Start DevBot with:\n  go run ./cmd/devbot\n\n", cfgPath)
	return nil
}

// ── Platform ──────────────────────────────────────────────────────────────────

func (w *Wizard) collectPlatform(a *answers) error {
	fmt.Println("Choose how you will send commands to DevBot:")
	fmt.Println("  telegram  — Telegram bot (recommended)")
	fmt.Println("  discord   — Discord bot")
	fmt.Println()

	for {
		v := w.prompt("Platform", "telegram")
		v = strings.ToLower(v)
		switch v {
		case "telegram":
			a.platform = v
			return w.collectTelegram(a)
		case "discord":
			a.platform = v
			return w.collectDiscord(a)
		default:
			fmt.Println("  Please enter 'telegram' or 'discord'.")
		}
	}
}

func (w *Wizard) collectTelegram(a *answers) error {
	fmt.Println()
	fmt.Println("Telegram setup:")
	fmt.Println("  1. Open Telegram and message @BotFather")
	fmt.Println("  2. Send /newbot and follow the prompts")
	fmt.Println("  3. Copy the token BotFather gives you")
	fmt.Println()

	a.tgToken = w.promptRequired("Bot token")

	fmt.Println()
	fmt.Println("  Find your Telegram user ID by messaging @userinfobot")
	fmt.Println("  Enter one user ID per line. Press Enter on a blank line when done.")
	fmt.Println()

	a.tgUserIDs = w.promptList("Your Telegram user ID")
	return nil
}

func (w *Wizard) collectDiscord(a *answers) error {
	fmt.Println()
	fmt.Println("Discord setup:")
	fmt.Println("  1. Go to https://discord.com/developers/applications → New Application")
	fmt.Println("  2. Go to Bot → Add Bot → copy the token")
	fmt.Println("  3. Under Privileged Gateway Intents, enable Message Content Intent")
	fmt.Println("  4. Under OAuth2 → URL Generator, select bot scope + Send Messages permission")
	fmt.Println("  5. Invite the bot using the generated URL")
	fmt.Println()

	a.dsToken = w.promptRequired("Bot token")
	a.dsPrefix = w.prompt("Command prefix", "!")

	fmt.Println()
	fmt.Println("  Enable Developer Mode in Discord: Settings → Advanced → Developer Mode")
	fmt.Println("  Right-click your username → Copy User ID")
	fmt.Println("  Enter one user ID per line. Press Enter on a blank line when done.")
	fmt.Println()

	a.dsUserIDs = w.promptList("Your Discord user ID")
	return nil
}

// ── Git Identity ──────────────────────────────────────────────────────────────

func (w *Wizard) collectGitIdentity(a *answers) error {
	fmt.Println("Name and email used when DevBot commits code.")
	fmt.Println("Use your GitHub-verified email so commits show as Verified on GitHub.")
	fmt.Println()

	a.gitName = w.prompt("Name", "DevBot")
	a.gitEmail = w.prompt("Email", "devbot@users.noreply.github.com")
	return nil
}

// ── GitHub ────────────────────────────────────────────────────────────────────

func (w *Wizard) collectGitHub(a *answers) error {
	fmt.Println("Create a GitHub Personal Access Token:")
	fmt.Println("  Settings → Developer settings → Personal access tokens → Fine-grained tokens")
	fmt.Println("  Grant: Contents (Read & Write), Pull requests (Read & Write)")
	fmt.Println()

	a.ghToken = w.promptRequired("GitHub token (ghp_...)")

	fmt.Println()
	fmt.Println("How many repositories should DevBot manage?")
	fmt.Println("  single — one repo (simpler config)")
	fmt.Println("  multi  — two or more repos (each task targets a named repo)")
	fmt.Println()

	for {
		mode := w.prompt("Mode", "single")
		switch strings.ToLower(mode) {
		case "single", "s":
			return w.collectSingleRepo(a)
		case "multi", "m", "multiple":
			return w.collectMultiRepo(a)
		default:
			fmt.Println("  Please enter 'single' or 'multi'.")
		}
	}
}

func (w *Wizard) collectSingleRepo(a *answers) error {
	fmt.Println()
	owner := w.promptRequired("Repository owner (username or org)")
	repo := w.promptRequired("Repository name")
	base := w.prompt("Base branch", "main")
	a.repos = []repoEntry{{owner: owner, repo: repo, baseBranch: base}}
	return nil
}

func (w *Wizard) collectMultiRepo(a *answers) error {
	fmt.Println()
	fmt.Println("Add repositories one at a time. Enter a blank repo name to stop.")
	fmt.Println()

	for i := 1; ; i++ {
		fmt.Printf("  Repository %d\n", i)
		owner := w.promptRequired("    Owner")
		repo := w.promptRequired("    Repo name")
		name := w.prompt("    Short alias (used in /task add <alias> \"...\")", strings.ToLower(repo))
		base := w.promptRequired("    Base branch (required per repo in multi-repo mode)")

		a.repos = append(a.repos, repoEntry{
			owner:      owner,
			repo:       repo,
			name:       name,
			baseBranch: base,
		})

		fmt.Println()
		if !w.confirm("Add another repository?") {
			break
		}
		fmt.Println()
	}
	return nil
}

// ── AI Provider ───────────────────────────────────────────────────────────────

func (w *Wizard) collectAI(a *answers) error {
	fmt.Println("Supported providers:")
	fmt.Println("  claude  — Anthropic Claude (recommended) — console.anthropic.com")
	fmt.Println("  openai  — OpenAI GPT — platform.openai.com")
	fmt.Println("  gemini  — Google Gemini — aistudio.google.com")
	fmt.Println("  local   — Local model via Ollama / LM Studio / LocalAI (no API key)")
	fmt.Println("  codex   — OpenAI Codex via ChatGPT subscription (no API key)")
	fmt.Println()

	for {
		p := strings.ToLower(w.prompt("Provider", "claude"))
		switch p {
		case "claude", "openai", "gemini", "local", "codex":
			a.aiProvider = p
			return w.collectProviderCredentials(a)
		default:
			fmt.Println("  Please choose: claude, openai, gemini, local, or codex.")
		}
	}
}

func (w *Wizard) collectProviderCredentials(a *answers) error {
	fmt.Println()
	switch a.aiProvider {
	case "claude":
		a.claudeKey = w.promptRequired("Anthropic API key (sk-ant-...)")
		a.claudeModel = w.prompt("Model", "claude-sonnet-4-6")

	case "openai":
		a.openaiKey = w.promptRequired("OpenAI API key (sk-...)")
		a.openaiModel = w.prompt("Model", "gpt-4o")

	case "gemini":
		a.geminiKey = w.promptRequired("Gemini API key (AIzaSy...)")
		a.geminiModel = w.prompt("Model", "gemini-1.5-pro")

	case "local":
		fmt.Println("  Common base URLs:")
		fmt.Println("    Ollama:    http://localhost:11434/v1")
		fmt.Println("    LM Studio: http://localhost:1234/v1")
		fmt.Println("    LocalAI:   http://localhost:8080/v1")
		fmt.Println()
		a.localURL = w.prompt("Base URL", "http://localhost:11434/v1")
		a.localModel = w.promptRequired("Model name (e.g. llama3.2, mistral)")

	case "codex":
		fmt.Println("  Requires a ChatGPT Plus, Pro, or Team subscription.")
		fmt.Println("  Run: npm install -g @openai/codex && codex login")
		fmt.Println("  DevBot reads ~/.codex/auth.json automatically after login.")
		fmt.Println()
		a.codexModel = w.prompt("Model", "codex-mini-latest")
	}
	return nil
}

// ── Config writer ─────────────────────────────────────────────────────────────

func (w *Wizard) writeConfig(path string, a *answers) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	defer f.Close()

	wf := func(format string, args ...any) {
		fmt.Fprintf(f, format, args...)
	}

	wf("bot:\n  platform: %q\n\n", a.platform)

	switch a.platform {
	case "telegram":
		wf("telegram:\n  token: %q\n  allowed_user_ids:\n", a.tgToken)
		for _, id := range a.tgUserIDs {
			wf("    - %s\n", id)
		}
	case "discord":
		wf("discord:\n  token: %q\n  command_prefix: %q\n  allowed_user_ids:\n", a.dsToken, a.dsPrefix)
		for _, id := range a.dsUserIDs {
			wf("    - %q\n", id)
		}
	}

	wf("\ngit:\n  name: %q\n  email: %q\n", a.gitName, a.gitEmail)

	wf("\ngithub:\n  token: %q\n", a.ghToken)
	if len(a.repos) == 1 {
		r := a.repos[0]
		wf("  owner: %q\n  repo: %q\n  base_branch: %q\n", r.owner, r.repo, r.baseBranch)
	} else {
		wf("  repos:\n")
		for _, r := range a.repos {
			wf("    - owner: %q\n      repo: %q\n      name: %q\n      base_branch: %q\n",
				r.owner, r.repo, r.name, r.baseBranch)
		}
	}

	wf("\nai:\n  provider: %q\n", a.aiProvider)

	switch a.aiProvider {
	case "claude":
		wf("\nclaude:\n  api_key: %q\n  model: %q\n", a.claudeKey, a.claudeModel)
	case "openai":
		wf("\nopenai:\n  api_key: %q\n  model: %q\n", a.openaiKey, a.openaiModel)
	case "gemini":
		wf("\ngemini:\n  api_key: %q\n  model: %q\n", a.geminiKey, a.geminiModel)
	case "local":
		wf("\nlocal:\n  base_url: %q\n  model: %q\n", a.localURL, a.localModel)
	case "codex":
		wf("\ncodex:\n  model: %q\n", a.codexModel)
	}

	wf("\ndatabase:\n  path: \"./devbot.db\"\n")
	wf("\n# Uncomment to enable the auto-scheduler (processes tasks Mon-Fri during work hours):\n")
	wf("# schedule:\n#   enabled: true\n#   timezone: \"UTC\"\n#   work_start: \"09:00\"\n#   work_end: \"17:00\"\n#   check_interval_minutes: 10\n")

	return nil
}

// ── Input helpers ─────────────────────────────────────────────────────────────

// prompt shows label with an optional default and returns the trimmed input.
// If the user presses Enter without typing, def is returned.
func (w *Wizard) prompt(label, def string) string {
	if def != "" {
		fmt.Printf("  %s [%s]: ", label, def)
	} else {
		fmt.Printf("  %s: ", label)
	}
	line, _ := w.r.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return def
	}
	return line
}

// promptRequired loops until the user provides a non-empty value.
func (w *Wizard) promptRequired(label string) string {
	for {
		v := w.prompt(label, "")
		if v != "" {
			return v
		}
		fmt.Printf("  %s is required.\n", label)
	}
}

// promptList collects one value per line until the user submits a blank line.
// Duplicate entries are rejected with a message.
func (w *Wizard) promptList(label string) []string {
	var items []string
	seen := make(map[string]struct{})
	for {
		v := w.prompt(label, "")
		if v == "" {
			if len(items) == 0 {
				fmt.Println("  At least one entry is required.")
				continue
			}
			break
		}
		if _, err := strconv.ParseInt(v, 10, 64); err != nil {
			fmt.Println("  User IDs must be numeric. Please try again.")
			continue
		}
		if _, dup := seen[v]; dup {
			fmt.Printf("  %q is already in the list, skipping.\n", v)
			continue
		}
		seen[v] = struct{}{}
		items = append(items, v)
	}
	return items
}

// confirm shows a yes/no prompt and returns true for y/yes.
func (w *Wizard) confirm(question string) bool {
	v := w.prompt(question+" (y/n)", "n")
	return strings.ToLower(v) == "y" || strings.ToLower(v) == "yes"
}

// ── Display helpers ───────────────────────────────────────────────────────────

func (w *Wizard) header() {
	fmt.Println("DevBot — First-Run Setup")
	fmt.Println("========================")
}

func (w *Wizard) section(title string) {
	fmt.Println(title)
	fmt.Println(strings.Repeat("-", len(title)))
}
