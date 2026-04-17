package bot

import (
	"context"
	"fmt"
	"strings"
)

type schedWizardStep int

const (
	schedStepTimezone  schedWizardStep = iota
	schedStepWorkStart schedWizardStep = iota
	schedStepWorkEnd   schedWizardStep = iota
)

type schedWizardSession struct {
	step      schedWizardStep
	timezone  string
	workStart string
	workEnd   string
}

func (b *Bot) startScheduleWizard(sessionKey string, notify func(string)) {
	cfg := b.sched.Config()
	b.startSchedWizard(sessionKey, &schedWizardSession{
		step:      schedStepTimezone,
		timezone:  cfg.Timezone,
		workStart: cfg.WorkStart,
		workEnd:   cfg.WorkEnd,
	})
	notify(fmt.Sprintf(
		"Configure auto-scheduler\n\nStep 1/3 — Timezone\nEnter an IANA timezone name.\nExamples: UTC, Asia/Bangkok, America/New_York, Europe/London\n\nCurrent: %s\n\n(/skip to keep current, /cancel to quit)",
		cfg.Timezone,
	))
}

func (b *Bot) stepSchedWizard(ctx context.Context, sessionKey string, wiz *schedWizardSession, text string, notify func(string)) {
	if strings.EqualFold(strings.TrimSpace(text), "/cancel") {
		b.endSchedWizard(sessionKey)
		notify("Setup cancelled.")
		return
	}

	switch wiz.step {
	case schedStepTimezone:
		if !isSkip(text) {
			wiz.timezone = strings.TrimSpace(text)
		}
		wiz.step = schedStepWorkStart
		notify(fmt.Sprintf(
			"Step 2/3 — Work start time\nWhen should DevBot start picking up tasks? (Mon-Fri only)\nFormat: HH:MM (24-hour), e.g. 09:00\n\nCurrent: %s\n\n(/skip to keep current)",
			wiz.workStart,
		))

	case schedStepWorkStart:
		if !isSkip(text) {
			wiz.workStart = strings.TrimSpace(text)
		}
		wiz.step = schedStepWorkEnd
		notify(fmt.Sprintf(
			"Step 3/3 — Work end time\nWhen should DevBot stop picking up new tasks?\nFormat: HH:MM (24-hour), e.g. 17:00\n\nCurrent: %s\n\n(/skip to keep current)",
			wiz.workEnd,
		))

	case schedStepWorkEnd:
		if !isSkip(text) {
			wiz.workEnd = strings.TrimSpace(text)
		}
		b.endSchedWizard(sessionKey)
		b.commitSchedWizard(wiz, notify)
	}
}

func (b *Bot) commitSchedWizard(wiz *schedWizardSession, notify func(string)) {
	if err := b.sched.Reconfigure(wiz.timezone, wiz.workStart, wiz.workEnd); err != nil {
		notify(fmt.Sprintf("Invalid configuration: %v\n\nRun /schedule setup to try again.", err))
		return
	}
	notify(fmt.Sprintf(
		"Scheduler updated!\n\nTimezone:   %s\nWork hours: Mon-Fri %s-%s\n\nChanges are live immediately.\nTo persist after restart, update config.yaml:\n\n  schedule:\n    timezone: \"%s\"\n    work_start: \"%s\"\n    work_end: \"%s\"",
		wiz.timezone, wiz.workStart, wiz.workEnd,
		wiz.timezone, wiz.workStart, wiz.workEnd,
	))
}
