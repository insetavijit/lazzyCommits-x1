package state

import (
	"context"
	"sync"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/lazycommit/lazycommit/internal/config"
	"github.com/lazycommit/lazycommit/internal/scheduler"
	"github.com/lazycommit/lazycommit/internal/watcher"
	"go.uber.org/zap"
)

type State string

const (
	IDLE     State = "IDLE"
	DIRTY    State = "DIRTY"
	UNPUSHED State = "UNPUSHED"
)

type StateMachine struct {
	repoCfg      config.RepoConfig
	scheduler    *scheduler.Scheduler
	logger       *zap.Logger
	events       <-chan watcher.Event

	state        State
	mu           sync.RWMutex

	idleDuration time.Duration
	pushDuration time.Duration
}

func NewStateMachine(
	repoCfg config.RepoConfig,
	globalCfg config.GlobalConfig,
	sched *scheduler.Scheduler,
	events <-chan watcher.Event,
	logger *zap.Logger,
) *StateMachine {
	idleMin := globalCfg.IdleBeforeCommitMinutes
	if repoCfg.Overrides.IdleBeforeCommitMinutes > 0 {
		idleMin = repoCfg.Overrides.IdleBeforeCommitMinutes
	}

	pushSec := globalCfg.PushDelaySeconds
	if repoCfg.Overrides.PushDelaySeconds > 0 {
		pushSec = repoCfg.Overrides.PushDelaySeconds
	}

	return &StateMachine{
		repoCfg:      repoCfg,
		scheduler:    sched,
		events:       events,
		logger:       logger.With(zap.String("repo", repoCfg.Path)),
		state:        IDLE,
		idleDuration: time.Duration(idleMin) * time.Minute,
		pushDuration: time.Duration(pushSec) * time.Second,
	}
}

func (sm *StateMachine) Run(ctx context.Context) {
	sm.logger.Info("State machine started", zap.String("state", string(sm.GetState())))

	sm.initialSweep()

	for {
		select {
		case event := <-sm.events:
			sm.handleEvent(event)
		case <-ctx.Done():
			sm.scheduler.RemoveRepoTask(sm.repoCfg.Path, scheduler.TaskCommit)
			sm.scheduler.RemoveRepoTask(sm.repoCfg.Path, scheduler.TaskPush)
			return
		}
	}
}

func (sm *StateMachine) initialSweep() {
	sm.logger.Info("Performing initial sweep for leftover stale files")
	
	// We check for stale files here simply to kickstart the DIRTY state if needed.
	// If it's dirty, we transition to DIRTY and the scheduler takes over.
	repo, err := git.PlainOpen(sm.repoCfg.Path)
	if err != nil {
		return
	}
	wt, err := repo.Worktree()
	if err != nil {
		return
	}
	status, err := wt.Status()
	if err != nil {
		return
	}

	hasChanges := false
	for _, s := range status {
		if s.Worktree == git.Modified || s.Worktree == git.Deleted || s.Worktree == git.Untracked {
			hasChanges = true
			break
		}
	}

	if hasChanges {
		sm.transitionTo(DIRTY)
		sm.scheduleCommit()
	}
}

func (sm *StateMachine) ScheduleManualCommit(delay time.Duration, msg string) {
	sm.logger.Info("Scheduling manual commit via Orchestrator", zap.Duration("delay", delay), zap.String("message", msg))
	sm.scheduler.UpsertRepoTask(sm.repoCfg.Path, scheduler.TaskCommit, time.Now().Add(delay), []string{msg})
	sm.transitionTo(DIRTY) // Assume it's dirty if we are scheduling a commit
}

func (sm *StateMachine) handleEvent(event watcher.Event) {
	state := sm.GetState()
	sm.logger.Debug("Received event", zap.String("type", string(event.Type)), zap.String("state", string(state)))

	switch event.Type {
	case watcher.FileChanged:
		sm.onFileChanged()
	case watcher.CommitDetected:
		sm.onCommitDetected()
	}
}

func (sm *StateMachine) onFileChanged() {
	switch sm.GetState() {
	case IDLE, DIRTY, UNPUSHED:
		sm.transitionTo(DIRTY)
		if sm.repoCfg.LazyCommit {
			sm.scheduleCommit()
		}
		sm.scheduler.RemoveRepoTask(sm.repoCfg.Path, scheduler.TaskPush)
	}
}

func (sm *StateMachine) onCommitDetected() {
	switch sm.GetState() {
	case IDLE, DIRTY, UNPUSHED:
		sm.transitionTo(UNPUSHED)
		if sm.repoCfg.LazyPush {
			sm.schedulePush()
		}
		sm.scheduler.RemoveRepoTask(sm.repoCfg.Path, scheduler.TaskCommit)
	}
}

func (sm *StateMachine) scheduleCommit() {
	sm.scheduler.UpsertRepoTask(sm.repoCfg.Path, scheduler.TaskCommit, time.Now().Add(sm.idleDuration), []string{"auto-commit"})
}

func (sm *StateMachine) schedulePush() {
	sm.scheduler.UpsertRepoTask(sm.repoCfg.Path, scheduler.TaskPush, time.Now().Add(sm.pushDuration), nil)
}

func (sm *StateMachine) transitionTo(newState State) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if sm.state == newState {
		return
	}
	sm.logger.Info("Transition", zap.String("from", string(sm.state)), zap.String("to", string(newState)))
	sm.state = newState
}

func (sm *StateMachine) GetState() State {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.state
}
