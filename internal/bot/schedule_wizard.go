package bot

import (
	"context"
	"fmt"
	"strings"
)

type SchedWizardStep int

const (
	schedStepTimezone      SchedWizardStep = iota
	schedStepWorkStart     SchedWizardStep = iota
	schedStepWorkEnd       SchedWizardStep = iota
	schedStepEnableWeekend SchedWizardStep = iota
)

type schedWizardSession struct {
	step          SchedWizardStep
	timezone      string
	workStart     string
	workEnd       string
	enableWeekend bool
}

func (b *Bot) startScheduleWizard(sessionKey string, notify func(string)) {
	cfg := b.sched.Config()
	b.startSchedWizard(sessionKey, &schedWizardSession{
		step:          schedStepTimezone,
		timezone:      cfg.Timezone,
		workStart:     cfg.WorkStart,
		workEnd:       cfg.WorkEnd,
		enableWeekend: cfg.EnableWeekend,
	})
	notify(fmt.Sprintf(
		"Configure auto-scheduler\n\nStep 1/4 — Timezone\nEnter an IANA timezone name.\nExamples: UTC, Asia/Bangkok, America/New_York, Europe/London\n\nCurrent: %s\n\n(/skip to keep current, /cancel to quit)",
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
			"Step 2/4 — Work start time\nWhen should DevBot start picking up tasks?\nFormat: HH:MM (24-hour), e.g. 09:00\n\nCurrent: %s\n\n(/skip to keep current)",
			wiz.workStart,
		))

	case schedStepWorkStart:
		if !isSkip(text) {
			wiz.workStart = strings.TrimSpace(text)
		}
		wiz.step = schedStepWorkEnd
		notify(fmt.Sprintf(
			"Step 3/4 — Work end time\nWhen should DevBot stop picking up new tasks?\nFormat: HH:MM (24-hour), e.g. 17:00\n\nCurrent: %s\n\n(/skip to keep current)",
			wiz.workEnd,
		))

	case schedStepWorkEnd:
		if !isSkip(text) {
			wiz.workEnd = strings.TrimSpace(text)
		}
		wiz.step = schedStepEnableWeekend
		current := "disabled"
		if wiz.enableWeekend {
			current = "enabled"
		}
		notify(fmt.Sprintf(
			"Step 4/4 — Weekend processing\nShould DevBot process tasks on Saturday and Sunday?\nReply: yes or no\n\nCurrent: %s\n\n(/skip to keep current)",
			current,
		))

	case schedStepEnableWeekend:
		if !isSkip(text) {
			switch strings.ToLower(strings.TrimSpace(text)) {
			case "yes", "y", "true", "1", "on":
				wiz.enableWeekend = true
			case "no", "n", "false", "0", "off":
				wiz.enableWeekend = false
			default:
				notify("Please reply yes or no.")
				return
			}
		}
		b.endSchedWizard(sessionKey)
		b.commitSchedWizard(wiz, notify)
	}
}

func (b *Bot) commitSchedWizard(wiz *schedWizardSession, notify func(string)) {
	if err := b.sched.Reconfigure(wiz.timezone, wiz.workStart, wiz.workEnd, wiz.enableWeekend); err != nil {
		notify(fmt.Sprintf("Invalid configuration: %v\n\nRun /schedule setup to try again.", err))
		return
	}
	weekendStr := "No"
	if wiz.enableWeekend {
		weekendStr = "Yes"
	}
	days := "Mon-Fri"
	if wiz.enableWeekend {
		days = "Mon-Sun"
	}
	notify(fmt.Sprintf(
		"Scheduler updated!\n\nTimezone:   %s\nWork hours: %s %s-%s\nWeekend:    %s\n\nChanges are live immediately.\nTo persist after restart, update config.yaml:\n\n  schedule:\n    timezone: \"%s\"\n    work_start: \"%s\"\n    work_end: \"%s\"\n    enable_weekend: %t",
		wiz.timezone, days, wiz.workStart, wiz.workEnd, weekendStr,
		wiz.timezone, wiz.workStart, wiz.workEnd, wiz.enableWeekend,
	))
}
