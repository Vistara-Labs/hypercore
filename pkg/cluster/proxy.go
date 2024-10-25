package cluster

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"
)

type ServiceProxy struct {
	mu                *sync.Mutex
	logger            *log.Logger
	proxiedPortMap    map[uint32]struct{}
	serviceIDPortMaps map[string]map[uint32]string
}

func NewServiceProxy(logger *log.Logger) (*ServiceProxy, error) {
	s := &ServiceProxy{
		logger:            logger,
		mu:                &sync.Mutex{},
		proxiedPortMap:    make(map[uint32]struct{}),
		serviceIDPortMaps: make(map[string]map[uint32]string),
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		split := strings.Split(r.Host, ".")
		if len(split) < 2 {
			s.logger.Warnf("bad host header: %s", r.Host)

			return
		}
		//nolint:nestif
		if portMap, ok := s.serviceIDPortMaps[split[0]]; ok {
			if containerAddr, ok := portMap[80]; ok {
				serverConn, err := net.Dial("tcp", containerAddr)
				if err != nil {
					s.logger.WithError(err).Errorf("failed to dial container %s at %s", split[0], containerAddr)

					return
				}

				go s.proxyConns(r.Body, serverConn, w)
			} else {
				s.logger.Warnf("no port mapped for %d for service %s", 80, split[0])
			}
		} else {
			s.logger.Warnf("no service found for identifier: %s", split[0])
		}
	})

	return s, nil
}

func (s *ServiceProxy) proxyConns(body io.Reader, server net.Conn, writer io.Writer) {
	errChan := make(chan<- struct{}, 1)
	cp := func(dst io.Writer, src io.Reader) {
		_, err := io.Copy(dst, src)
		// The connection that breaks first can do the error handling and
		// close both of them
		select {
		case errChan <- struct{}{}:
			s.logger.WithError(err).Errorf("disconnected from %s", server.LocalAddr())
			server.Close()
		default:
		}
	}

	go cp(server, body)
	cp(writer, server)
}

func (s *ServiceProxy) Register(hostPort uint32, containerID, containerAddr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.proxiedPortMap[hostPort]; ok {
		if _, ok := s.serviceIDPortMaps[containerID]; !ok {
			s.serviceIDPortMaps[containerID] = make(map[uint32]string)
		}
		s.serviceIDPortMaps[containerID][hostPort] = containerAddr

		return nil
	}

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", hostPort))
	if err != nil {
		return fmt.Errorf("failed to listen on port %d: %w", hostPort, err)
	}

	go func() {
		defer func() {
			s.mu.Lock()
			_ = listener.Close()
			delete(s.proxiedPortMap, hostPort)
			s.mu.Unlock()
		}()

		if err := http.Serve(listener, nil); err != nil {
			s.logger.WithError(err).Errorf("failed to serve HTTP at port %d", hostPort)
		}
	}()

	return nil
}
