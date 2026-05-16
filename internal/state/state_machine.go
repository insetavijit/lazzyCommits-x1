package state

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/lazycommit/lazycommit/internal/config"
	"github.com/lazycommit/lazycommit/internal/git"
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
	engine         git.GitEngine
	guard          *git.SafetyGuard
	activityLogger *logger.ActivityLogger
	logger         *zap.Logger
	events         <-chan watcher.Event

	state State
	mu    sync.RWMutex

	idleTimer *time.Timer
	pushTimer *time.Timer

	idleDuration time.Duration
	pushDuration time.Duration

	retryQueue *queue.RetryQueue

	// Manual schedule
	manualTimer *time.Timer
	manualMsg   string
}

func NewStateMachine(
	repoCfg config.RepoConfig,
	globalCfg config.GlobalConfig,
	engine git.GitEngine,
	guard *git.SafetyGuard,
	activityLogger *logger.ActivityLogger,
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
		activityLogger: activityLogger,
		events:         events,
		logger:         logger.With(zap.String("repo", repoCfg.Path)),
		state:          IDLE,
		idleDuration:   time.Duration(idleMin) * time.Minute,
		pushDuration:   time.Duration(pushSec) * time.Second,
		retryQueue:     retryQueue,
	}
}

func (sm *StateMachine) Run(ctx context.Context) {
	sm.logger.Info("State machine started", zap.String("state", string(sm.GetState())))

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
		case <-ctx.Done():
			sm.stopTimers()
			return
		}
	}
}

func (sm *StateMachine) ScheduleManualCommit(delay time.Duration, msg string) {
	sm.logger.Info("Scheduling manual commit", zap.Duration("delay", delay), zap.String("message", msg))
	sm.stopManualTimer()
	sm.manualMsg = msg
	sm.manualTimer = time.NewTimer(delay)
}

func (sm *StateMachine) manualTimerChan() <-chan time.Time {
	if sm.manualTimer == nil {
		return nil
	}
	return sm.manualTimer.C
}

func (sm *StateMachine) stopManualTimer() {
	if sm.manualTimer != nil {
		sm.manualTimer.Stop()
		sm.manualTimer = nil
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
	res := sm.guard.Check(sm.repoCfg.Path, sm.repoCfg.ProtectedBranches, 50) // TODO: use global max file size
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
		sm.transitionTo(IDLE) // Go back to IDLE, will retry on next change or manual fix
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
	if sm.idleTimer == nil {
		return nil
	}
	return sm.idleTimer.C
}

func (sm *StateMachine) pushTimerChan() <-chan time.Time {
	if sm.pushTimer == nil {
		return nil
	}
	return sm.pushTimer.C
}

func (sm *StateMachine) resetIdleTimer() {
	sm.stopIdleTimer()
	sm.idleTimer = time.NewTimer(sm.idleDuration)
}

func (sm *StateMachine) stopIdleTimer() {
	if sm.idleTimer != nil {
		sm.idleTimer.Stop()
		sm.idleTimer = nil
	}
}

func (sm *StateMachine) resetPushTimer() {
	sm.stopPushTimer()
	sm.pushTimer = time.NewTimer(sm.pushDuration)
}

func (sm *StateMachine) stopPushTimer() {
	if sm.pushTimer != nil {
		sm.pushTimer.Stop()
		sm.pushTimer = nil
	}
}

func (sm *StateMachine) stopTimers() {
	sm.stopIdleTimer()
	sm.stopPushTimer()
}
