package bot

import (
	"context"
	"fmt"
	"strings"
)

type wizardStep int

const (
	wizardStepTitle wizardStep = iota
	wizardStepDescription
	wizardStepRepo
	wizardStepTechStack
)

type wizardSession struct {
	step        wizardStep
	title       string
	description string
	repoOwner   string
	repoName    string
}

// startCreateWizard begins a guided /task create session for the given user.
func (b *Bot) startCreateWizard(ctx context.Context, sessionKey string, notify func(string)) {
	b.startWizard(sessionKey, &wizardSession{step: wizardStepTitle})
	notify("What's the title for your task?\n\n(Send /cancel at any point to quit.)")
}

// stepWizard advances the wizard one step based on the user's reply.
func (b *Bot) stepWizard(ctx context.Context, sessionKey string, wiz *wizardSession, text string, notify func(string)) {
	if strings.EqualFold(strings.TrimSpace(text), "/cancel") {
		b.endWizard(sessionKey)
		notify("Wizard cancelled.")
		return
	}

	switch wiz.step {
	case wizardStepTitle:
		title := strings.TrimSpace(text)
		if title == "" {
			notify("Title cannot be empty. What's the title for your task?")
			return
		}
		wiz.title = title
		wiz.step = wizardStepDescription
		notify("Got it! Add a description (or /skip to leave it blank):")

	case wizardStepDescription:
		if !isSkip(text) {
			wiz.description = strings.TrimSpace(text)
		}
		if b.pool.IsMultiRepo() {
			wiz.step = wizardStepRepo
			notify(b.repoPrompt())
		} else {
			wiz.step = wizardStepTechStack
			notify("Any tech stack or constraints? (or /skip):")
		}

	case wizardStepRepo:
		input := strings.TrimSpace(text)
		var owner, name string
		if isSkip(input) || strings.EqualFold(input, "default") {
			def := b.pool.Default()
			if def != nil {
				owner, name = def.Owner(), def.Repo()
			}
		} else {
			c := b.pool.Lookup(input)
			if c == nil {
				notify(fmt.Sprintf("Unknown repository %q.\n\n%s", input, b.repoPrompt()))
				return
			}
			owner, name = c.Owner(), c.Repo()
		}
		wiz.repoOwner = owner
		wiz.repoName = name
		wiz.step = wizardStepTechStack
		notify("Any tech stack or constraints? (or /skip):")

	case wizardStepTechStack:
		var techStack string
		if !isSkip(text) {
			techStack = strings.TrimSpace(text)
		}
		b.endWizard(sessionKey)
		b.commitWizard(ctx, wiz, techStack, notify)
	}
}

// repoPrompt builds the repository selection message shown during the wizard.
func (b *Bot) repoPrompt() string {
	var sb strings.Builder
	sb.WriteString("Which repository?\n")
	for _, c := range b.pool.All() {
		if c.Name() != "" {
			sb.WriteString(fmt.Sprintf("\n• %s  (%s/%s)", c.Name(), c.Owner(), c.Repo()))
		} else {
			sb.WriteString(fmt.Sprintf("\n• %s/%s", c.Owner(), c.Repo()))
		}
	}
	sb.WriteString("\n\nReply with the alias (e.g. backend) or owner/repo. Send /skip for the default.")
	return sb.String()
}

// commitWizard creates the task from the completed wizard state.
func (b *Bot) commitWizard(ctx context.Context, wiz *wizardSession, techStack string, notify func(string)) {
	desc := wiz.description
	if techStack != "" {
		if desc != "" {
			desc += "\n\nTech stack: " + techStack
		} else {
			desc = "Tech stack: " + techStack
		}
	}

	owner, name := wiz.repoOwner, wiz.repoName
	if owner == "" {
		if def := b.pool.Default(); def != nil {
			owner, name = def.Owner(), def.Repo()
		}
	}

	t, err := b.taskSvc.Add(ctx, wiz.title, desc, owner, name)
	if err != nil {
		notify(fmt.Sprintf("Error creating task: %v", err))
		return
	}

	repoLabel := ""
	if b.pool.IsMultiRepo() {
		repoLabel = fmt.Sprintf(" [%s/%s]", t.RepoOwner, t.RepoName)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Task %d created%s!\n\nTitle: %s", t.ID, repoLabel, t.Title))
	if wiz.description != "" {
		sb.WriteString(fmt.Sprintf("\nDescription: %s", wiz.description))
	}
	if techStack != "" {
		sb.WriteString(fmt.Sprintf("\nTech stack: %s", techStack))
	}
	sb.WriteString(fmt.Sprintf("\n\nStart it with: /task do %d", t.ID))
	notify(sb.String())
}

func isSkip(s string) bool {
	return strings.EqualFold(strings.TrimSpace(s), "/skip")
}
