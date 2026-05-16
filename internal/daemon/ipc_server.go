package daemon

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/lazycommit/lazycommit/internal/ipc"
	"github.com/lazycommit/lazycommit/internal/scheduler"
	"go.uber.org/zap"
)

type IPCServer struct {
	registry  *Registry
	scheduler *scheduler.Scheduler
	logger    *zap.Logger
	socket    string
}

func NewIPCServer(registry *Registry, sched *scheduler.Scheduler, logger *zap.Logger) (*IPCServer, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	socket := filepath.Join(home, ".lazycommit", "daemon.sock")

	return &IPCServer{
		registry:  registry,
		scheduler: sched,
		logger:    logger,
		socket:    socket,
	}, nil
}

func (s *IPCServer) Start(ctx context.Context) error {
	// Delete existing socket file
	if _, err := os.Stat(s.socket); err == nil {
		if err := os.Remove(s.socket); err != nil {
			return err
		}
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(s.socket), 0755); err != nil {
		return err
	}

	listener, err := net.Listen("unix", s.socket)
	if err != nil {
		return err
	}

	s.logger.Info("IPC server started", zap.String("socket", s.socket))

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				select {
				case <-ctx.Done():
					return
				default:
					s.logger.Error("Failed to accept connection", zap.Error(err))
					continue
				}
			}
			go s.handleConnection(conn)
		}
	}()

	go func() {
		<-ctx.Done()
		listener.Close()
		os.Remove(s.socket)
		s.logger.Info("IPC server stopped and socket removed")
	}()

	return nil
}

func (s *IPCServer) handleConnection(conn net.Conn) {
	defer conn.Close()

	var envelope struct {
		Type    string          `json:"type"`
		Payload json.RawMessage `json:"payload"`
	}
	if err := json.NewDecoder(conn).Decode(&envelope); err != nil {
		return
	}

	var response interface{}

	switch envelope.Type {
	case "status":
		resp := ipc.StatusResponse{}
		for _, repo := range s.registry.All() {
			stateStr := "UNKNOWN"
			var scheduledAt time.Time
			var scheduledMsg string
			if repo.StateMachine != nil {
				stateStr = string(repo.StateMachine.GetState())
				scheduledAt, scheduledMsg = repo.StateMachine.GetScheduledInfo()
			}
			resp.Repos = append(resp.Repos, ipc.RepoStatus{
				Path:         repo.Config.Path,
				State:        stateStr,
				ScheduledAt:  scheduledAt,
				ScheduledMsg: scheduledMsg,
			})
		}
		response = resp

	case "task":
		var req ipc.TaskRequest
		if err := json.Unmarshal(envelope.Payload, &req); err != nil {
			response = ipc.TaskResponse{Success: false, Error: "invalid payload"}
		} else {
			response = s.handleTask(req)
		}

	case "scheduled_list":
		tasks := s.scheduler.All()
		var list []ipc.TaskInfo
		for _, t := range tasks {
			list = append(list, ipc.TaskInfo{
				ID:    t.ID,
				Type:  string(t.Type),
				Repo:  t.Repo,
				RunAt: t.RunAt,
			})
		}
		response = ipc.ScheduledListResponse{Tasks: list}
	}

	if response != nil {
		if err := json.NewEncoder(conn).Encode(response); err != nil {
			s.logger.Error("Failed to encode IPC response", zap.Error(err))
		}
	}
}

func (s *IPCServer) handleTask(req ipc.TaskRequest) ipc.TaskResponse {
	delay, err := time.ParseDuration(req.Delay)
	if err != nil {
		return ipc.TaskResponse{Success: false, Error: "invalid duration format"}
	}

	task := &scheduler.Task{
		Type:  scheduler.TaskType(req.Type),
		Repo:  req.Repo,
		Args:  req.Args,
		RunAt: time.Now().Add(delay),
	}

	if err := s.scheduler.Schedule(task); err != nil {
		return ipc.TaskResponse{Success: false, Error: err.Error()}
	}

	return ipc.TaskResponse{Success: true, ID: task.ID}
}
