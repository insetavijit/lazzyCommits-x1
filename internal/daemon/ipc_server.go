package daemon

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/lazycommit/lazycommit/internal/ipc"
	"go.uber.org/zap"
)

type IPCServer struct {
	registry *Registry
	logger   *zap.Logger
	socket   string
}

func NewIPCServer(registry *Registry, logger *zap.Logger) (*IPCServer, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	socket := filepath.Join(home, ".lazycommit", "daemon.sock")

	return &IPCServer{
		registry: registry,
		logger:   logger,
		socket:   socket,
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

	case "schedule":
		var req ipc.ScheduleRequest
		if err := json.Unmarshal(envelope.Payload, &req); err != nil {
			response = ipc.ScheduleResponse{Success: false, Error: "invalid payload"}
		} else {
			response = s.handleSchedule(req)
		}
	}

	if response != nil {
		if err := json.NewEncoder(conn).Encode(response); err != nil {
			s.logger.Error("Failed to encode IPC response", zap.Error(err))
		}
	}
}

func (s *IPCServer) handleSchedule(req ipc.ScheduleRequest) ipc.ScheduleResponse {
	repo, ok := s.registry.Get(req.RepoPath)
	if !ok {
		return ipc.ScheduleResponse{Success: false, Error: "repository not found or not being watched"}
	}

	if repo.StateMachine == nil {
		return ipc.ScheduleResponse{Success: false, Error: "state machine not initialized for this repository"}
	}

	importTime, err := time.ParseDuration(req.Delay)
	if err != nil {
		return ipc.ScheduleResponse{Success: false, Error: "invalid duration format"}
	}

	repo.StateMachine.ScheduleManualCommit(importTime, req.Message)
	return ipc.ScheduleResponse{Success: true}
}
