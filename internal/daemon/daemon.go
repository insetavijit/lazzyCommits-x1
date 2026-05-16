package daemon

import (
	"context"
	"os"
	"os/signal"
	"reflect"
	"syscall"
	"time"

	"github.com/lazycommit/lazycommit/internal/config"
	"github.com/lazycommit/lazycommit/internal/git"
	actlogger "github.com/lazycommit/lazycommit/internal/logger"
	"github.com/lazycommit/lazycommit/internal/queue"
	"github.com/lazycommit/lazycommit/internal/scheduler"
	"github.com/lazycommit/lazycommit/internal/state"
	"github.com/lazycommit/lazycommit/internal/watcher"
	"go.uber.org/zap"
)

type Daemon struct {
	cfg            *config.Config
	logger         *zap.Logger
	registry       *Registry
	ctx            context.Context
	retryQueue     *queue.RetryQueue
	activityLogger *actlogger.ActivityLogger
	scheduler      *scheduler.Scheduler
}

func New(cfg *config.Config, logger *zap.Logger) *Daemon {
	rq, _ := queue.NewRetryQueue(logger)
	sched, _ := scheduler.NewScheduler(logger)
	return &Daemon{
		cfg:            cfg,
		logger:         logger,
		registry:       NewRegistry(),
		retryQueue:     rq,
		activityLogger: actlogger.NewActivityLogger(),
		scheduler:      sched,
	}
}

func (d *Daemon) Start(ctx context.Context) error {
	d.ctx = ctx
	d.logger.Info("Starting lazyCommit daemon", zap.Int("num_repos_config", len(d.cfg.Repos)))

	// Initialize registry from config
	for _, repoCfg := range d.cfg.Repos {
		d.logger.Info("Checking repo config", zap.String("path", repoCfg.Path), zap.Bool("enabled", repoCfg.Enabled))
		if repoCfg.Enabled {
			repoCtx, cancel := context.WithCancel(d.ctx)
			d.registry.Add(repoCfg, cancel)
			repo, _ := d.registry.Get(repoCfg.Path)
			go d.watchRepo(repoCtx, repo)
		}
	}

	go d.watchConfig(d.ctx)
	go d.pollRetryQueue(d.ctx)
	go d.pollScheduler(d.ctx)

	ipcServer, err := NewIPCServer(d.registry, d.scheduler, d.logger)
	if err == nil {
		ipcServer.Start(d.ctx)
	} else {
		d.logger.Error("Failed to initialize IPC server", zap.Error(err))
	}

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigChan:
		d.logger.Info("Received signal, shutting down", zap.String("signal", sig.String()))
	case <-ctx.Done():
		d.logger.Info("Context cancelled, shutting down")
	}

	return nil
}

func (d *Daemon) pollScheduler(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	engine := git.NewEngine(d.logger)

	for {
		select {
		case <-ticker.C:
			pending := d.scheduler.GetPending()
			for _, task := range pending {
				d.logger.Info("Executing scheduled task", 
					zap.String("id", task.ID), 
					zap.String("type", string(task.Type)))
				
				switch task.Type {
				case scheduler.TaskCommit:
					msg := ""
					if len(task.Args) > 0 {
						msg = task.Args[0]
					}
					err := engine.StageAndCommitWithMsg(task.Repo, msg)
					if err != nil {
						d.logger.Error("Scheduled commit failed", zap.Error(err))
					}
				case scheduler.TaskPush:
					err := engine.Push(task.Repo)
					if err != nil {
						d.logger.Error("Scheduled push failed", zap.Error(err))
						d.retryQueue.Enqueue(task.Repo)
					}
				}
			}
		case <-ctx.Done():
			return
		}
	}
}

func (d *Daemon) pollRetryQueue(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	engine := git.NewEngine(d.logger)

	for {
		select {
		case <-ticker.C:
			item := d.retryQueue.PeekReady()
			if item == nil {
				continue
			}

			// Check if repo still exists in registry
			if _, exists := d.registry.Get(item.RepoPath); !exists {
				d.logger.Debug("Skipping retry for removed/disabled repo", zap.String("repo", item.RepoPath))
				continue
			}

			d.logger.Info("Retrying push", zap.String("repo", item.RepoPath))
			err := engine.Push(item.RepoPath)
			if err != nil {
				d.logger.Warn("Retry push failed", zap.String("repo", item.RepoPath), zap.Error(err))
				d.retryQueue.HandleFailure(item)
			} else {
				d.logger.Info("Retry push successful", zap.String("repo", item.RepoPath))
			}
		case <-ctx.Done():
			return
		}
	}
}

func (d *Daemon) Reconcile(newCfg *config.Config) {
	d.logger.Info("Reconciling registry with new config")

	newReposMap := make(map[string]config.RepoConfig)
	for _, repoCfg := range newCfg.Repos {
		newReposMap[repoCfg.Path] = repoCfg
	}

	// 1. Handle removals and updates
	for _, existingRepo := range d.registry.All() {
		path := existingRepo.Config.Path
		newRepoCfg, exists := newReposMap[path]

		if !exists || !newRepoCfg.Enabled {
			d.logger.Info("Stopping repo watcher (removed or disabled)", zap.String("path", path))
			existingRepo.Cancel()
			d.registry.Remove(path)
			continue
		}

		if !reflect.DeepEqual(existingRepo.Config, newRepoCfg) || !reflect.DeepEqual(d.cfg.Global, newCfg.Global) {
			d.logger.Info("Config changed for repo (or global config changed), restarting watcher", zap.String("path", path))
			existingRepo.Cancel()
			d.registry.Remove(path)
			// Will be re-added in the next loop
		}
	}

	// 2. Handle new additions
	for path, newRepoCfg := range newReposMap {
		if !newRepoCfg.Enabled {
			continue
		}
		if _, exists := d.registry.Get(path); !exists {
			d.logger.Info("Starting repo watcher", zap.String("path", path))
			repoCtx, cancel := context.WithCancel(d.ctx)
			d.registry.Add(newRepoCfg, cancel)
			repo, _ := d.registry.Get(path)
			go d.watchRepo(repoCtx, repo)
		}
	}

	d.cfg = newCfg
}

func (d *Daemon) watchRepo(ctx context.Context, repo *Repository) {
	d.logger.Info("Watching repository", zap.String("path", repo.Config.Path))
	
	rw := watcher.NewRepoWatcher(repo.Config.Path, d.logger)
	engine := git.NewEngine(d.logger)
	guard := git.NewSafetyGuard(d.logger)

	go rw.Watch(ctx)

	sm := state.NewStateMachine(
		repo.Config,
		d.cfg.Global,
		engine,
		guard,
		d.activityLogger,
		rw.Events(),
		d.retryQueue,
		d.logger,
	)

	d.registry.SetStateMachine(repo.Config.Path, sm)

	sm.Run(ctx)
}
