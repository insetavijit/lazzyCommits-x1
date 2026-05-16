package daemon

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"

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

	var req ipc.StatusRequest
	// We use Decode because we expect a JSON object. 
	// If the client just connects and closes, this might error.
	if err := json.NewDecoder(conn).Decode(&req); err != nil {
		// It's possible the client just wants a status without sending anything, 
		// but our protocol says they send StatusRequest.
		return 
	}

	resp := ipc.StatusResponse{}
	for _, repo := range s.registry.All() {
		stateStr := "UNKNOWN"
		if repo.StateMachine != nil {
			stateStr = string(repo.StateMachine.GetState())
		}
		resp.Repos = append(resp.Repos, ipc.RepoStatus{
			Path:  repo.Config.Path,
			State: stateStr,
		})
	}

	if err := json.NewEncoder(conn).Encode(resp); err != nil {
		s.logger.Error("Failed to encode IPC response", zap.Error(err))
	}
}
