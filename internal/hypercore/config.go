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
	ClusterPolicyFile    string
	GrpcBindAddr         string
	HTTPBindAddr         string
	BeaconEndpoint       string
	BeaconPrice          float64
	BeaconReputation     string
	ClusterSpawn         struct {
		CPU        int
		Memory     int
		ImageRef   string
		Ports      string
		Env        []string
		PolicyFile string
	}
	ClusterStop struct {
		ID string
	}
	ClusterLogs struct {
		ID string
	}
}
