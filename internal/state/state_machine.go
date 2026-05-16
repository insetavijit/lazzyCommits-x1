package state

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/lazycommit/lazycommit/internal/config"
	internalgit "github.com/lazycommit/lazycommit/internal/git"
	"github.com/lazycommit/lazycommit/internal/logger"
	"github.com/lazycommit/lazycommit/internal/queue"
	"github.com/lazycommit/lazycommit/internal/watcher"
	"go.uber.org/zap"
)

type State string

const (
	IDLE       State = "IDLE"
	DIRTY      State = "DIRTY"
	COMMITTING State = "COMMITTING"
	UNPUSHED   State = "UNPUSHED"
	PUSHING    State = "PUSHING"
	PAUSED     State = "PAUSED"
	ERROR      State = "ERROR"
)

type StateMachine struct {
	repoCfg        config.RepoConfig
	engine         internalgit.GitEngine
	guard          *internalgit.SafetyGuard
	activityLogger *logger.ActivityLogger
	logger         *zap.Logger
	events         <-chan watcher.Event
	retryQueue     *queue.RetryQueue

	state State
	mu    sync.RWMutex

	idleTimer *time.Timer
	pushTimer *time.Timer

	idleDuration time.Duration
	pushDuration time.Duration

	// Manual schedule
	manualTimer *time.Timer
	manualMsg   string
	manualAt    time.Time
}

func NewStateMachine(
	repoCfg config.RepoConfig,
	globalCfg config.GlobalConfig,
	engine internalgit.GitEngine,
	guard *internalgit.SafetyGuard,
	actLogger *logger.ActivityLogger,
	events <-chan watcher.Event,
	retryQueue *queue.RetryQueue,
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
		repoCfg:        repoCfg,
		engine:         engine,
		guard:          guard,
		activityLogger: actLogger,
		events:         events,
		retryQueue:     retryQueue,
		logger:         logger.With(zap.String("repo", repoCfg.Path)),
		state:          IDLE,
		idleDuration:   time.Duration(idleMin) * time.Minute,
		pushDuration:   time.Duration(pushSec) * time.Second,
	}
}

func (sm *StateMachine) Run(ctx context.Context) {
	sm.logger.Info("State machine started", zap.String("state", string(sm.GetState())))

	// Perform initial sweep for stale files
	sm.initialSweep()

	// Start heartbeat for stale files
	heartbeat := time.NewTicker(1 * time.Minute)
	defer heartbeat.Stop()

	for {
		select {
		case event := <-sm.events:
			sm.handleEvent(event)
		case <-sm.idleTimerChan():
			sm.handleIdleTimer()
		case <-sm.pushTimerChan():
			sm.handlePushTimer()
		case <-sm.manualTimerChan():
			sm.handleManualTimer()
		case <-heartbeat.C:
			sm.heartbeat()
		case <-ctx.Done():
			sm.stopTimers()
			sm.stopManualTimer()
			return
		}
	}
}

func (sm *StateMachine) initialSweep() {
	sm.logger.Info("Performing initial sweep for leftover stale files")
	sm.heartbeat()
}

func (sm *StateMachine) heartbeat() {
	// Find tracked files older than 15 minutes
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

	var staleFiles []string
	threshold := time.Now().Add(-15 * time.Minute)

	for path, s := range status {
		// Only consider tracked modified/deleted files
		if s.Worktree == git.Modified || s.Worktree == git.Deleted {
			absPath := filepath.Join(sm.repoCfg.Path, path)
			info, err := os.Stat(absPath)
			if err == nil {
				if info.ModTime().Before(threshold) {
					staleFiles = append(staleFiles, path)
				}
			}
		}
	}

	if len(staleFiles) > 0 {
		sm.logger.Info("Found stale files, performing proactive commit", zap.Int("num_files", len(staleFiles)))
		msg := fmt.Sprintf("auto: proactive checkpoint for %d stale files", len(staleFiles))
		start := time.Now()
		err := sm.engine.StageAndCommitFiles(sm.repoCfg.Path, staleFiles, msg)
		if err == nil {
			sm.activityLogger.Log(sm.repoCfg.Path, logger.ActionCommitted, logger.OutcomeSuccess, time.Since(start), nil)
			sm.transitionTo(UNPUSHED)
			sm.resetPushTimer()
		} else {
			sm.logger.Error("Proactive stale commit failed", zap.Error(err))
		}
	}
}

func (sm *StateMachine) ScheduleManualCommit(delay time.Duration, msg string) {
	sm.logger.Info("Scheduling manual commit", zap.Duration("delay", delay), zap.String("message", msg))
	sm.stopManualTimer()
	sm.mu.Lock()
	sm.manualMsg = msg
	sm.manualAt = time.Now().Add(delay)
	sm.manualTimer = time.NewTimer(delay)
	sm.mu.Unlock()
}

func (sm *StateMachine) GetScheduledInfo() (time.Time, string) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	if sm.manualTimer == nil {
		return time.Time{}, ""
	}
	return sm.manualAt, sm.manualMsg
}

func (sm *StateMachine) manualTimerChan() <-chan time.Time {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	if sm.manualTimer == nil {
		return nil
	}
	return sm.manualTimer.C
}

func (sm *StateMachine) stopManualTimer() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if sm.manualTimer != nil {
		sm.manualTimer.Stop()
		sm.manualTimer = nil
		sm.manualAt = time.Time{}
	}
}

func (sm *StateMachine) handleManualTimer() {
	sm.logger.Info("Manual commit timer fired")
	sm.stopManualTimer()
	sm.transitionTo(COMMITTING)
	sm.performCommitWithMsg(sm.manualMsg)
}

func (sm *StateMachine) performCommitWithMsg(msg string) {
	start := time.Now()
	sm.logger.Info("Performing manual auto-commit", zap.String("msg", msg))

	// Safety Guard check
	res := sm.guard.Check(sm.repoCfg.Path, sm.repoCfg.ProtectedBranches, 50)
	if !res.Passed {
		err := fmt.Errorf("safety guard failed: %s", res.Reason)
		sm.logger.Warn("Safety guard failed, aborting manual commit", zap.String("reason", res.Reason))
		sm.activityLogger.Log(sm.repoCfg.Path, logger.ActionCommitted, logger.OutcomeFailed, time.Since(start), err)
		sm.transitionTo(IDLE)
		return
	}

	err := sm.engine.StageAndCommitWithMsg(sm.repoCfg.Path, msg)
	if err != nil {
		sm.logger.Error("Manual auto-commit failed", zap.Error(err))
		sm.activityLogger.Log(sm.repoCfg.Path, logger.ActionCommitted, logger.OutcomeFailed, time.Since(start), err)
		sm.transitionTo(IDLE)
		return
	}

	sm.activityLogger.Log(sm.repoCfg.Path, logger.ActionCommitted, logger.OutcomeSuccess, time.Since(start), nil)
	sm.transitionTo(UNPUSHED)
	sm.resetPushTimer()
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
	case IDLE, DIRTY:
		sm.transitionTo(DIRTY)
		sm.resetIdleTimer()
	case UNPUSHED:
		sm.stopPushTimer()
		sm.transitionTo(DIRTY)
		sm.resetIdleTimer()
	}
}

func (sm *StateMachine) onCommitDetected() {
	switch sm.GetState() {
	case IDLE, DIRTY, UNPUSHED:
		sm.stopIdleTimer()
		sm.transitionTo(UNPUSHED)
		sm.resetPushTimer()
	}
}

func (sm *StateMachine) handleIdleTimer() {
	if sm.GetState() != DIRTY {
		return
	}

	if !sm.repoCfg.LazyCommit {
		sm.logger.Info("Idle timer fired but Lazy Commit is disabled")
		sm.transitionTo(IDLE)
		return
	}

	sm.transitionTo(COMMITTING)
	sm.performCommit()
}

func (sm *StateMachine) handlePushTimer() {
	if sm.GetState() != UNPUSHED {
		return
	}

	if !sm.repoCfg.LazyPush {
		sm.logger.Info("Push timer fired but Lazy Push is disabled")
		sm.transitionTo(IDLE)
		return
	}

	sm.transitionTo(PUSHING)
	sm.performPush()
}

func (sm *StateMachine) performCommit() {
	start := time.Now()
	sm.logger.Info("Performing auto-commit")

	// Safety Guard check
	res := sm.guard.Check(sm.repoCfg.Path, sm.repoCfg.ProtectedBranches, 50)
	if !res.Passed {
		err := fmt.Errorf("safety guard failed: %s", res.Reason)
		sm.logger.Warn("Safety guard failed, aborting commit", zap.String("reason", res.Reason))
		sm.activityLogger.Log(sm.repoCfg.Path, logger.ActionCommitted, logger.OutcomeFailed, time.Since(start), err)
		sm.transitionTo(IDLE)
		return
	}

	err := sm.engine.StageAndCommit(sm.repoCfg.Path)
	if err != nil {
		sm.logger.Error("Auto-commit failed", zap.Error(err))
		sm.activityLogger.Log(sm.repoCfg.Path, logger.ActionCommitted, logger.OutcomeFailed, time.Since(start), err)
		sm.transitionTo(IDLE)
		return
	}

	sm.activityLogger.Log(sm.repoCfg.Path, logger.ActionCommitted, logger.OutcomeSuccess, time.Since(start), nil)
	sm.transitionTo(UNPUSHED)
	sm.resetPushTimer()
}

func (sm *StateMachine) performPush() {
	start := time.Now()
	sm.logger.Info("Performing auto-push")

	// Safety Guard check
	res := sm.guard.Check(sm.repoCfg.Path, sm.repoCfg.ProtectedBranches, 50)
	if !res.Passed {
		err := fmt.Errorf("safety guard failed: %s", res.Reason)
		sm.logger.Warn("Safety guard failed, aborting push", zap.String("reason", res.Reason))
		sm.activityLogger.Log(sm.repoCfg.Path, logger.ActionPushed, logger.OutcomeFailed, time.Since(start), err)
		sm.transitionTo(IDLE)
		return
	}

	err := sm.engine.Push(sm.repoCfg.Path)
	if err != nil {
		sm.logger.Error("Auto-push failed, enqueuing to retry queue", zap.Error(err))
		sm.activityLogger.Log(sm.repoCfg.Path, logger.ActionPushed, logger.OutcomeFailed, time.Since(start), err)
		if sm.retryQueue != nil {
			sm.retryQueue.Enqueue(sm.repoCfg.Path)
		}
		sm.transitionTo(IDLE)
		return
	}

	sm.activityLogger.Log(sm.repoCfg.Path, logger.ActionPushed, logger.OutcomeSuccess, time.Since(start), nil)
	sm.transitionTo(IDLE)
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

func (sm *StateMachine) idleTimerChan() <-chan time.Time {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	if sm.idleTimer == nil {
		return nil
	}
	return sm.idleTimer.C
}

func (sm *StateMachine) pushTimerChan() <-chan time.Time {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	if sm.pushTimer == nil {
		return nil
	}
	return sm.pushTimer.C
}

func (sm *StateMachine) resetIdleTimer() {
	sm.stopIdleTimer()
	sm.mu.Lock()
	sm.idleTimer = time.NewTimer(sm.idleDuration)
	sm.mu.Unlock()
}

func (sm *StateMachine) stopIdleTimer() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if sm.idleTimer != nil {
		sm.idleTimer.Stop()
		sm.idleTimer = nil
	}
}

func (sm *StateMachine) resetPushTimer() {
	sm.stopPushTimer()
	sm.mu.Lock()
	sm.pushTimer = time.NewTimer(sm.pushDuration)
	sm.mu.Unlock()
}

func (sm *StateMachine) stopPushTimer() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if sm.pushTimer != nil {
		sm.pushTimer.Stop()
		sm.pushTimer = nil
	}
}

func (sm *StateMachine) stopTimers() {
	sm.stopIdleTimer()
	sm.stopPushTimer()
}
