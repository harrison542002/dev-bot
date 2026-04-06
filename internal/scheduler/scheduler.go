package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"devbot/internal/agent"
	"devbot/internal/config"
	"devbot/internal/store"
	"devbot/internal/task"
)

// Scheduler polls the task queue during configured work hours (Mon-Fri by
// default) and automatically runs the agent on TODO tasks, one at a time.
type Scheduler struct {
	cfg       *config.ScheduleConfig
	svc       *task.Service
	ag        *agent.Agent
	loc       *time.Location
	workStart int // minutes from midnight, e.g. 540 for 09:00
	workEnd   int // minutes from midnight, e.g. 1020 for 17:00

	mu          sync.Mutex
	broadcast   func(string) // send message to all users; set via SetBroadcast
	paused      bool
	agentBusy   bool   // true while an agent goroutine is executing
	lastRunDate string // "2006-01-02" on which morning briefing was last sent
}

// New creates a Scheduler. broadcast may be nil initially; call SetBroadcast
// before Start to avoid missed notifications.
func New(cfg *config.ScheduleConfig, svc *task.Service, ag *agent.Agent, broadcast func(string)) (*Scheduler, error) {
	loc, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		return nil, fmt.Errorf("invalid schedule.timezone %q: %w", cfg.Timezone, err)
	}
	workStart, err := parseHHMM(cfg.WorkStart)
	if err != nil {
		return nil, fmt.Errorf("invalid schedule.work_start %q: %w", cfg.WorkStart, err)
	}
	workEnd, err := parseHHMM(cfg.WorkEnd)
	if err != nil {
		return nil, fmt.Errorf("invalid schedule.work_end %q: %w", cfg.WorkEnd, err)
	}
	if workStart >= workEnd {
		return nil, fmt.Errorf("schedule.work_start (%s) must be before work_end (%s)", cfg.WorkStart, cfg.WorkEnd)
	}
	return &Scheduler{
		cfg:       cfg,
		svc:       svc,
		ag:        ag,
		loc:       loc,
		workStart: workStart,
		workEnd:   workEnd,
		broadcast: broadcast,
	}, nil
}

// SetBroadcast wires in the bot's broadcast function after construction.
// Must be called before Start.
func (s *Scheduler) SetBroadcast(fn func(string)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.broadcast = fn
}

// Start runs the scheduler loop. Blocks until ctx is cancelled.
func (s *Scheduler) Start(ctx context.Context) {
	interval := time.Duration(s.cfg.CheckIntervalMinutes) * time.Minute
	slog.Info("scheduler started",
		"timezone", s.cfg.Timezone,
		"work_start", s.cfg.WorkStart,
		"work_end", s.cfg.WorkEnd,
		"check_interval", interval,
	)

	// Fire immediately on startup so we don't wait a full interval.
	s.tick(ctx)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("scheduler stopped")
			return
		case <-ticker.C:
			s.tick(ctx)
		}
	}
}

// Pause suspends automatic task processing (in-memory; resets on restart).
func (s *Scheduler) Pause() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.paused = true
}

// Resume re-enables automatic task processing.
func (s *Scheduler) Resume() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.paused = false
}

// IsPaused reports whether the scheduler is manually paused.
func (s *Scheduler) IsPaused() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.paused
}

// IsEnabled reports whether the scheduler is enabled in config.
func (s *Scheduler) IsEnabled() bool {
	return s.cfg.Enabled
}

// IsAgentBusy reports whether an agent task is currently executing.
func (s *Scheduler) IsAgentBusy() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.agentBusy
}

// Status returns a human-readable status string for the /schedule command.
func (s *Scheduler) Status(ctx context.Context) string {
	s.mu.Lock()
	paused := s.paused
	busy := s.agentBusy
	s.mu.Unlock()

	active := s.isWorkTime()
	now := time.Now().In(s.loc)

	var activeStr string
	if paused {
		activeStr = "PAUSED"
	} else if active {
		activeStr = "ACTIVE"
	} else {
		wd := now.Weekday()
		if wd == time.Saturday || wd == time.Sunday {
			activeStr = "INACTIVE (weekend)"
		} else {
			activeStr = "INACTIVE (outside work hours)"
		}
	}

	todoCount := 0
	if tasks, err := s.svc.List(ctx); err == nil {
		for _, t := range tasks {
			if t.Status == store.StatusTodo {
				todoCount++
			}
		}
	}

	return fmt.Sprintf(
		"Auto-Scheduler\n\nEnabled: %s\nPaused: %s\nWork window: Mon-Fri %s-%s (%s)\nCheck interval: every %d minutes\nRight now: %s\nAgent running: %s\n\nTODO queue: %d tasks",
		boolYN(s.cfg.Enabled),
		boolYN(paused),
		s.cfg.WorkStart,
		s.cfg.WorkEnd,
		s.cfg.Timezone,
		s.cfg.CheckIntervalMinutes,
		activeStr,
		boolYN(busy),
		todoCount,
	)
}

// tick is called on each timer fire. Holds the mutex only for state checks;
// releases it before launching the agent goroutine to avoid blocking.
func (s *Scheduler) tick(ctx context.Context) {
	s.mu.Lock()

	if s.paused || s.agentBusy || !s.isWorkTimeUnlocked() {
		s.mu.Unlock()
		return
	}

	// Morning briefing — once per work day
	s.sendMorningBriefingUnlocked(ctx)

	// Find the oldest TODO task
	tasks, err := s.svc.List(ctx)
	if err != nil {
		s.mu.Unlock()
		slog.Error("scheduler: failed to list tasks", "err", err)
		return
	}

	var next *store.Task
	for _, t := range tasks {
		if t.Status == store.StatusTodo {
			next = t
			break
		}
	}

	if next == nil {
		s.mu.Unlock()
		return // nothing to process
	}

	s.agentBusy = true
	taskID := next.ID
	taskTitle := next.Title
	broadcast := s.broadcast
	s.mu.Unlock()

	if broadcast != nil {
		broadcast(fmt.Sprintf("Auto-starting task %d: %s", taskID, taskTitle))
	}

	go func() {
		defer func() {
			s.mu.Lock()
			s.agentBusy = false
			s.mu.Unlock()
		}()
		s.ag.Run(ctx, taskID, func(msg string) {
			s.mu.Lock()
			fn := s.broadcast
			s.mu.Unlock()
			if fn != nil {
				fn(msg)
			}
		})
	}()
}

// isWorkTimeUnlocked checks the work window. Caller must hold mu.
func (s *Scheduler) isWorkTimeUnlocked() bool {
	now := time.Now().In(s.loc)
	wd := now.Weekday()
	if wd == time.Saturday || wd == time.Sunday {
		return false
	}
	h, m, _ := now.Clock()
	current := h*60 + m
	return current >= s.workStart && current < s.workEnd
}

// isWorkTime is the lock-safe version for external callers (Status).
func (s *Scheduler) isWorkTime() bool {
	return s.isWorkTimeUnlocked()
}

// sendMorningBriefingUnlocked sends the daily morning greeting on the first
// tick of each work day. Caller must hold mu.
func (s *Scheduler) sendMorningBriefingUnlocked(ctx context.Context) {
	today := time.Now().In(s.loc).Format("2006-01-02")
	if s.lastRunDate == today {
		return
	}
	s.lastRunDate = today

	tasks, err := s.svc.List(ctx)
	if err != nil {
		return
	}

	todoCount := 0
	var titles []string
	for _, t := range tasks {
		if t.Status == store.StatusTodo {
			todoCount++
			if len(titles) < 3 {
				titles = append(titles, fmt.Sprintf("  • #%d %s", t.ID, t.Title))
			}
		}
	}

	if s.broadcast == nil {
		return
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Good morning! Work day started.\n\n%d task(s) in queue.", todoCount))
	if len(titles) > 0 {
		sb.WriteString("\n\nNext up:\n")
		sb.WriteString(strings.Join(titles, "\n"))
		if todoCount > len(titles) {
			sb.WriteString(fmt.Sprintf("\n  ... and %d more", todoCount-len(titles)))
		}
	} else {
		sb.WriteString("\n\nNothing to do — add tasks with /task add.")
	}
	s.broadcast(sb.String())
}

// parseHHMM converts "HH:MM" to minutes from midnight.
func parseHHMM(s string) (int, error) {
	var h, m int
	if _, err := fmt.Sscanf(s, "%d:%d", &h, &m); err != nil {
		return 0, fmt.Errorf("expected HH:MM format")
	}
	if h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, fmt.Errorf("time out of range (0-23:0-59)")
	}
	return h*60 + m, nil
}

func boolYN(b bool) string {
	if b {
		return "Yes"
	}
	return "No"
}
