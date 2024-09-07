package cluster

import (
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
)

type Proxy struct {
	hostPort   uint32
	mappedPort uint32
	proxyConn  net.Listener
}

type ServiceProxy struct {
	logger                *log.Logger
	containerIDToProxyMap map[string][]Proxy
}

func NewServiceProxy(logger *log.Logger) (*ServiceProxy, error) {
	return &ServiceProxy{
		logger:                logger,
		containerIDToProxyMap: make(map[string][]Proxy),
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

func (s *ServiceProxy) GetPortMap(containerID string) (map[uint32]uint32, error) {
	proxies, ok := s.containerIDToProxyMap[containerID]
	if !ok {
		return nil, fmt.Errorf("no proxy found for container %s", containerID)
	}

	portMap := make(map[uint32]uint32)
	for _, proxy := range proxies {
		portMap[proxy.hostPort] = proxy.mappedPort
	}

	return portMap, nil
}

func (s *ServiceProxy) Register(containerID, containerAddr string) (int, error) {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return 0, err
	}

	hostPort, err := strconv.ParseUint(strings.Split(containerAddr, ":")[1], 10, 0)
	if err != nil {
		return 0, err
	}

	_, ok := s.containerIDToProxyMap[containerID]
	if !ok {
		s.containerIDToProxyMap[containerID] = make([]Proxy, 0)
	}
	s.containerIDToProxyMap[containerID] = append(s.containerIDToProxyMap[containerID], Proxy{
		hostPort:   uint32(hostPort),
		mappedPort: uint32(listener.Addr().(*net.TCPAddr).Port),
		proxyConn:  listener,
	})

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
