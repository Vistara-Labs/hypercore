package cluster

import (
	"io"
	"net"

	log "github.com/sirupsen/logrus"
)

type ProxyConn struct {
	listener net.Listener
}

type ServiceProxy struct {
	logger              *log.Logger
	containerToProxyMap map[string]ProxyConn
}

func NewServiceProxy(logger *log.Logger) (*ServiceProxy, error) {
	return &ServiceProxy{
		logger:              logger,
		containerToProxyMap: make(map[string]ProxyConn),
	}, nil
}

func (s *ServiceProxy) proxyConns(client, server net.Conn) {
	errChan := make(chan<- struct{}, 1)
	cp := func(dst, src net.Conn) {
		_, err := io.Copy(dst, src)
		// The connection that breaks first can do the error handling and
		// close both of them
		select {
		case errChan <- struct{}{}:
			s.logger.WithError(err).Errorf("disconnected from %s", client.RemoteAddr())
			client.Close()
			server.Close()
		default:
		}
	}

	go cp(client, server)
	cp(server, client)
}

func (s *ServiceProxy) Register(containerID, containerAddr string) (int, error) {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return 0, err
	}

	s.containerToProxyMap[containerAddr] = ProxyConn{listener: listener}

	go func() {
		for {
			clientConn, err := listener.Accept()
			if err != nil {
				s.logger.WithError(err).Error("failed to accept connection")

				break
			}

			serverConn, err := net.Dial("tcp", containerAddr)
			if err != nil {
				s.logger.WithError(err).Errorf("failed to dial container %s at %s for client %s", containerID, containerAddr, clientConn.RemoteAddr())
				clientConn.Close()

				continue
			}

			s.logger.Infof("accepted client connection %s to proxy to container %s at %s", clientConn.RemoteAddr(), containerID, containerAddr)
			go s.proxyConns(clientConn, serverConn)
		}
	}()

	return listener.Addr().(*net.TCPAddr).Port, nil
}
