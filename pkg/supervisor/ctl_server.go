package supervisor

import (
	"net"

	"go.uber.org/zap"

	"spm/pkg/config"
	"spm/pkg/logger"
	"spm/pkg/utils"
)

type spmServer struct {
	sv     *Supervisor
	sock   net.Listener
	logger *zap.SugaredLogger
}

func (s *spmServer) Listen() {
	defer func() {
		_ = s.sock.Close()
		close(utils.FinishChan)
	}()

SERVER:
	for {
		select {
		case <-utils.FinishChan:
			break SERVER
		default:
			{
				conn, err := s.sock.Accept()
				if err != nil {
					s.logger.Error(err)
					continue
				}

				session := NewSession(s.sv, conn)
				result := session.Handle()
				if result == ResponseShutdown {
					utils.FinishChan <- struct{}{}
				}
			}
		}
	}

	s.logger.Info("Supervisor server is stopped")
}

func StartServer(s *Supervisor) {
	socket, err := net.Listen("unix", config.GetConfig().Socket)
	if err != nil {
		panic(err)
	}

	server := &spmServer{
		sv:     s,
		sock:   socket,
		logger: logger.Logging("spm-daemon"),
	}

	server.Listen()
}
