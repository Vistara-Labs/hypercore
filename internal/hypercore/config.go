package hypercore

type Config struct {
	CtrSocketPath        string
	CtrNamespace         string
	DefaultVMProvider    string
	HACFile              string
	RespawnOnNodeFailure bool
	ClusterBindAddr      string
	ClusterBaseURL       string
	ClusterTLSCert       string
	ClusterTLSKey        string
	GrpcBindAddr         string
	HTTPBindAddr         string
	ClusterSpawn         struct {
		CPU      int
		Memory   int
		ImageRef string
		Ports    string
		Env      []string
	}
	ClusterStop struct {
		ID string
	}
	ClusterLogs struct {
		ID string
	}
}
