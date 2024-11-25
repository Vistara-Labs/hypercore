package cluster

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"
)

type ServiceProxy struct {
	mu                *sync.Mutex
	logger            *log.Logger
	tlsConfig         *TLSConfig
	proxiedPortMap    map[uint32]struct{}
	serviceIDPortMaps map[string]map[uint32]string
}

type TLSConfig struct {
	CertFile string
	KeyFile  string
}

func NewServiceProxy(logger *log.Logger, tlsConfig *TLSConfig) (*ServiceProxy, error) {
	s := &ServiceProxy{
		logger:            logger,
		tlsConfig:         tlsConfig,
		mu:                &sync.Mutex{},
		proxiedPortMap:    make(map[uint32]struct{}),
		serviceIDPortMaps: make(map[string]map[uint32]string),
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		addr := r.Context().Value(http.LocalAddrContextKey).(net.Addr)
		port, err := strconv.Atoi(strings.Split(addr.String(), ":")[1])
		if err != nil {
			panic(fmt.Errorf("bad address: %s", addr.String()))
		}

		splitHost := strings.Split(r.Host, ".")
		if len(splitHost) < 2 {
			s.logger.Warnf("bad host header: %s", r.Host)

			return
		}
		host := splitHost[0]
		s.logger.Infof("Got request for host %s port %d", host, port)
		//nolint:nestif
		if portMap, ok := s.serviceIDPortMaps[host]; ok {
			if containerAddr, ok := portMap[uint32(port)]; ok {
				s.logger.Infof("got address %s for service at host %s", containerAddr, r.Host)

				proxiedURL, err := url.Parse("http://" + containerAddr)
				if err != nil {
					// this should not happen
					panic(fmt.Errorf("failed to parse container address %s: %w", containerAddr, err))
				}

				// TODO construct once per URL
				httputil.NewSingleHostReverseProxy(proxiedURL).ServeHTTP(w, r)
			} else {
				s.logger.Warnf("no port mapped for %d for service %s", port, host)
			}
		} else {
			s.logger.Warnf("no service found for identifier: %s", host)
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

		if s.tlsConfig != nil {
			if err := http.ServeTLS(listener, nil, s.tlsConfig.CertFile, s.tlsConfig.KeyFile); err != nil {
				s.logger.WithError(err).Errorf("failed to serve HTTP TLS at port %d", hostPort)
			}
		} else {
			if err := http.Serve(listener, nil); err != nil {
				s.logger.WithError(err).Errorf("failed to serve HTTP at port %d", hostPort)
			}
		}
	}()

	return nil
}

func (s *ServiceProxy) Services() map[string][]uint32 {
	s.mu.Lock()
	defer s.mu.Unlock()
	services := make(map[string][]uint32)
	for service, val := range s.serviceIDPortMaps {
		services[service] = []uint32{}
		for port := range val {
			services[service] = append(services[service], port)
		}
	}

	return services
}
