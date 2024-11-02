package hypercore

type Config struct {
	CtrSocketPath     string
	CtrNamespace      string
	DefaultVMProvider string
	HACFile           string
	ClusterBindAddr   string
	ClusterBaseURL    string
	ClusterTLSCert    string
	ClusterTLSKey     string
	GrpcBindAddr      string
	ClusterSpawn      struct {
		CPU      int
		Memory   int
		ImageRef string
		Ports    string
	}
}
