package daemon

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"os/signal"
	"reflect"
	"syscall"
	"time"

	"github.com/lazycommit/lazycommit/cmd/core"
	"github.com/lazycommit/lazycommit/internal/config"
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
	scheduler      *scheduler.Scheduler
}

func New(cfg *config.Config, logger *zap.Logger) *Daemon {
	rq, _ := queue.NewRetryQueue(logger)
	sched, _ := scheduler.NewScheduler(logger)
	return &Daemon{
		cfg:        cfg,
		logger:     logger,
		registry:   NewRegistry(),
		retryQueue: rq,
		scheduler:  sched,
	}
}

func (d *Daemon) Start(ctx context.Context) error {
	d.ctx = ctx
	d.logger.Info("Starting lazyCommit daemon", zap.Int("num_repos_config", len(d.cfg.Repos)))

	// Initialize registry from config
	for _, repoCfg := range d.cfg.Repos {
		d.logger.Info("Checking repo config", zap.String("path", repoCfg.Path), zap.Bool("enabled", repoCfg.Enabled))
		if repoCfg.Enabled {
			healthy, reason := d.isRepoHealthy(repoCfg.Path)
			if !healthy {
				d.logger.Warn("Ignoring repository: health check failed", zap.String("path", repoCfg.Path), zap.String("reason", reason))
				continue
			}

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

func (d *Daemon) isRepoHealthy(path string) (bool, string) {
	executable, err := os.Executable()
	if err != nil {
		return false, "failed to locate executable"
	}

	cmd := exec.Command(executable, "validate", "--health", "--path", path)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, string(output)
	}

	var env core.ResponseEnvelope
	if err := json.Unmarshal(output, &env); err != nil {
		return false, "failed to parse validate output"
	}

	if env.Error != "" {
		return false, env.Error
	}

	if env.Data != nil {
		dataBytes, _ := json.Marshal(env.Data)
		var resp core.ValidateResponse
		if json.Unmarshal(dataBytes, &resp) == nil && resp.Health != nil {
			if !resp.Health.IsHealthy {
				msg := resp.Health.Message
				if msg == "" {
					msg = "unknown health error"
				}
				return false, msg
			}
		}
	}

	return true, ""
}

func (d *Daemon) pollScheduler(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	executable, _ := os.Executable()

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
					msg := "auto-commit"
					if len(task.Args) > 0 && task.Args[0] != "" {
						msg = task.Args[0]
					}
					
					cmd := exec.Command(executable, "commit", task.Repo, msg)
					output, err := cmd.CombinedOutput()
					if err != nil {
						d.logger.Error("Scheduled commit process failed", zap.Error(err), zap.String("output", string(output)))
					} else {
						var env core.ResponseEnvelope
						if json.Unmarshal(output, &env) == nil && env.Error != "" {
							d.logger.Error("Scheduled commit failed internally", zap.String("error", env.Error))
						} else {
							d.logger.Info("Scheduled commit successful", zap.String("repo", task.Repo))
						}
					}
					
				case scheduler.TaskPush:
					cmd := exec.Command(executable, "push", task.Repo)
					output, err := cmd.CombinedOutput()
					if err != nil {
						d.logger.Error("Scheduled push process failed", zap.Error(err), zap.String("output", string(output)))
						d.retryQueue.Enqueue(task.Repo)
					} else {
						var env core.ResponseEnvelope
						if json.Unmarshal(output, &env) == nil && env.Error != "" {
							d.logger.Error("Scheduled push failed internally", zap.String("error", env.Error))
							d.retryQueue.Enqueue(task.Repo)
						} else {
							d.logger.Info("Scheduled push successful", zap.String("repo", task.Repo))
							// We successfully pushed. Watcher handles IDLE transition.
						}
					}
					
				case scheduler.TaskRun:
					if len(task.Args) > 0 {
						cmdStr := task.Args[0]
						d.logger.Info("Executing scheduled shell command", zap.String("command", cmdStr))
						cmd := exec.Command("bash", "-c", cmdStr)
						cmd.Dir = task.Repo
						output, err := cmd.CombinedOutput()
						if err != nil {
							d.logger.Error("Scheduled shell command failed", 
								zap.String("command", cmdStr), 
								zap.Error(err), 
								zap.String("output", string(output)))
						} else {
							d.logger.Info("Scheduled shell command successful", 
								zap.String("command", cmdStr), 
								zap.String("output", string(output)))
						}
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

	executable, _ := os.Executable()

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
			cmd := exec.Command(executable, "push", item.RepoPath)
			output, err := cmd.CombinedOutput()
			
			if err != nil {
				d.logger.Warn("Retry push process failed", zap.String("repo", item.RepoPath), zap.Error(err), zap.String("output", string(output)))
				d.retryQueue.HandleFailure(item)
				continue
			}
			
			var env core.ResponseEnvelope
			if json.Unmarshal(output, &env) == nil && env.Error != "" {
				d.logger.Warn("Retry push failed internally", zap.String("repo", item.RepoPath), zap.String("error", env.Error))
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
			healthy, reason := d.isRepoHealthy(path)
			if !healthy {
				d.logger.Warn("Ignoring new repository: health check failed", zap.String("path", path), zap.String("reason", reason))
				continue
			}

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

	go rw.Watch(ctx)

	sm := state.NewStateMachine(
		repo.Config,
		d.cfg.Global,
		d.scheduler,
		rw.Events(),
		d.logger,
	)

	d.registry.SetStateMachine(repo.Config.Path, sm)

	sm.Run(ctx)
}
