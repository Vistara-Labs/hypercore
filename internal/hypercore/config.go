package hypercore

type Config struct {
	CtrSocketPath     string
	CtrNamespace      string
	DefaultVMProvider string
	HACFile           string
	ClusterBindAddr   string
	GrpcBindAddr      string
}
