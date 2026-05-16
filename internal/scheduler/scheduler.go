package scheduler

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"
)

type TaskType string

const (
	TaskCommit TaskType = "commit"
	TaskPush   TaskType = "push"
	TaskRun    TaskType = "run"
)

type Task struct {
	ID      string    `json:"id"`
	Type    TaskType  `json:"type"`
	Repo    string    `json:"repo"`
	Args    []string  `json:"args"`
	RunAt   time.Time `json:"runAt"`
}

type Scheduler struct {
	tasks    map[string]*Task
	mu       sync.Mutex
	filePath string
	logger   *zap.Logger
}

func NewScheduler(logger *zap.Logger) (*Scheduler, error) {
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, ".lazycommit", "scheduled-tasks.json")

	s := &Scheduler{
		tasks:    make(map[string]*Task),
		filePath: path,
		logger:   logger,
	}

	if err := s.load(); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *Scheduler) Schedule(task *Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task.ID = fmt.Sprintf("%d", time.Now().UnixNano())
	s.tasks[task.ID] = task
	
	s.logger.Info("Task scheduled", 
		zap.String("id", task.ID), 
		zap.String("type", string(task.Type)), 
		zap.Time("at", task.RunAt))

	return s.save()
}

func (s *Scheduler) UpsertRepoTask(repo string, taskType TaskType, runAt time.Time, args []string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Find existing task for this repo and type
	for id, task := range s.tasks {
		if task.Repo == repo && task.Type == taskType {
			task.RunAt = runAt
			task.Args = args
			s.logger.Info("Task updated", 
				zap.String("id", id), 
				zap.String("type", string(taskType)), 
				zap.Time("at", runAt))
			return id, s.save()
		}
	}

	// Not found, create new
	task := &Task{
		ID:    fmt.Sprintf("%d", time.Now().UnixNano()),
		Type:  taskType,
		Repo:  repo,
		Args:  args,
		RunAt: runAt,
	}
	s.tasks[task.ID] = task
	
	s.logger.Info("Task scheduled", 
		zap.String("id", task.ID), 
		zap.String("type", string(task.Type)), 
		zap.Time("at", task.RunAt))

	return task.ID, s.save()
}

func (s *Scheduler) RemoveRepoTask(repo string, taskType TaskType) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	deleted := false
	for id, task := range s.tasks {
		if task.Repo == repo && task.Type == taskType {
			delete(s.tasks, id)
			s.logger.Info("Task removed", zap.String("id", id), zap.String("type", string(taskType)))
			deleted = true
		}
	}

	if deleted {
		return s.save()
	}
	return nil
}

func (s *Scheduler) GetPending() []*Task {
	s.mu.Lock()
	defer s.mu.Unlock()

	var pending []*Task
	now := time.Now()
	for id, task := range s.tasks {
		if task.RunAt.Before(now) {
			pending = append(pending, task)
			delete(s.tasks, id)
		}
	}
	
	if len(pending) > 0 {
		s.save()
	}
	
	return pending
}

func (s *Scheduler) All() []*Task {
	s.mu.Lock()
	defer s.mu.Unlock()
	var all []*Task
	for _, t := range s.tasks {
		all = append(all, t)
	}
	return all
}

func (s *Scheduler) Terminate(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.tasks[id]; !exists {
		return fmt.Errorf("task with ID %s not found", id)
	}

	delete(s.tasks, id)
	s.logger.Info("Task terminated", zap.String("id", id))
	return s.save()
}

func (s *Scheduler) load() error {
	if _, err := os.Stat(s.filePath); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, &s.tasks)
}

func (s *Scheduler) save() error {
	data, err := json.MarshalIndent(s.tasks, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.filePath, data, 0644)
}
