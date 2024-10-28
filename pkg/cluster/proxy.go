package cluster

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
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
				s.logger.Infof("got address %s for service at host %s", containerAddr, r.Host)

				proxiedURL, err := url.Parse("http://" + containerAddr)
				if err != nil {
					// this should not happen
					panic(fmt.Errorf("failed to parse container address %s: %w", containerAddr, err))
				}

				httputil.NewSingleHostReverseProxy(proxiedURL).ServeHTTP(w, r)
			} else {
				s.logger.Warnf("no port mapped for %d for service %s", 80, split[0])
			}
		} else {
			s.logger.Warnf("no service found for identifier: %s", split[0])
		}
	})

	return s, nil
}

func (s *ServiceProxy) Register(hostPort uint32, containerID, containerAddr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.serviceIDPortMaps[containerID]; !ok {
		s.serviceIDPortMaps[containerID] = make(map[uint32]string)
	}
	s.serviceIDPortMaps[containerID][hostPort] = containerAddr

	s.logger.Infof("Exposed container ID %s Address %s at host port %d", containerID, containerAddr, hostPort)

	if _, ok := s.proxiedPortMap[hostPort]; ok {
		return nil
	}

	s.logger.Infof("Listening on host port: %d", hostPort)

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", hostPort))
	if err != nil {
		return fmt.Errorf("failed to listen on port %d: %w", hostPort, err)
	}

	s.proxiedPortMap[hostPort] = struct{}{}
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
