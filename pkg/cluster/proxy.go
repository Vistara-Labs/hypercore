package cluster

type ServiceProxy struct {
}

func NewServiceProxy() (*ServiceProxy, error) {
	return &ServiceProxy{}, nil
}

func (s *ServiceProxy) Register(containerID, containerIP string, port int) error {
	_ = containerID
	_ = containerIP
	_ = port

	return nil
}
