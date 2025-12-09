package supervisor

import (
	"net"
	"sync"

	"go.uber.org/zap"

	"spm/pkg/codec"
	"spm/pkg/config"
	"spm/pkg/logger"
	"spm/pkg/utils"
)

type spmServer struct {
	sv     *Supervisor
	wg     sync.WaitGroup
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

				s.wg.Add(1)
				go func(se *SpmSession) {
					defer s.wg.Done()

					result := se.Handle()
					if result == codec.ResponseShutdown {
						utils.FinishChan <- struct{}{}
					}
				}(session)
			}
		}
	}

	s.wg.Wait()
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
