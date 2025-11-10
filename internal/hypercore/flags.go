package hypercore

import (
	"fmt"
	"vistara-node/pkg/defaults"
	"vistara-node/pkg/hypervisor/firecracker"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const (
	hacFileFlag              = "hac"
	containerdSocketFlag     = "containerd-socket"
	containerdNamespace      = "containerd-ns"
	vmProviderFlag           = "provider"
	grpcBindAddrFlag         = "grpc-bind-addr"
	httpBindAddrFlag         = "http-bind-addr"
	clusterBindAddrFlag      = "cluster-bind-addr"
	clusterBaseURLFlag       = "cluster-base-url"
	clusterTLSCertFlag       = "cluster-tls-cert"
	clusterTLSKeyFlag        = "cluster-tls-key"
	clusterPolicyFileFlag    = "cluster-policy"
	respawnOnNodeFailureFlag = "respawn-on-node-failure"
	cpuFlag                  = "cpu"
	memoryFlag               = "mem"
	imageRefFlag             = "image-ref"
	portsFlag                = "ports"
	envFlag                  = "env"
	policyFileFlag           = "policy"
	idFlag                   = "id"
)

func AddCommonFlags(cmd *cobra.Command, cfg *Config) {
	cmd.Flags().StringVar(&cfg.DefaultVMProvider,
		vmProviderFlag,
		firecracker.HypervisorName,
		"VM Provider to use")

	cmd.Flags().StringVar(&cfg.HACFile,
		hacFileFlag,
		defaults.HACFile,
		"Path to hac.toml")

	cmd.Flags().StringVar(&cfg.CtrSocketPath,
		containerdSocketFlag,
		defaults.ContainerdSocket,
		"The path to the containerd socket.")

	cmd.Flags().StringVar(&cfg.CtrNamespace,
		containerdNamespace,
		defaults.ContainerdNamespace,
		"The name of the containerd namespace to use.")
}

func AddClusterFlags(cmd *cobra.Command, cfg *Config) {
	cmd.Flags().StringVar(&cfg.GrpcBindAddr, grpcBindAddrFlag, "0.0.0.0:8000", "GRPC Server bind address")
	cmd.Flags().StringVar(&cfg.HTTPBindAddr, httpBindAddrFlag, "0.0.0.0:8001", "HTTP Server bind address")
	cmd.Flags().StringVar(&cfg.ClusterBindAddr, clusterBindAddrFlag, ":7946", "Cluster bind address")
	cmd.Flags().StringVar(&cfg.ClusterBaseURL, clusterBaseURLFlag, "example.com", "Cluster base URL")
	cmd.Flags().StringVar(&cfg.ClusterTLSCert, clusterTLSCertFlag, "", "Cluster tls cert path")
	cmd.Flags().StringVar(&cfg.ClusterTLSKey, clusterTLSKeyFlag, "", "Cluster tls key path")
	cmd.Flags().StringVar(&cfg.ClusterPolicyFile, clusterPolicyFileFlag, "", "Path to IBRL policy file (JSON)")
	cmd.Flags().BoolVar(&cfg.RespawnOnNodeFailure, respawnOnNodeFailureFlag, false, "Whether this node monitors other cluster nodes and re-schedules their tasks on failure")
}

func AddClusterSpawnFlags(cmd *cobra.Command, cfg *Config) {
	cmd.Flags().StringVar(&cfg.GrpcBindAddr, grpcBindAddrFlag, "0.0.0.0:8000", "GRPC Server bind address")
	cmd.Flags().IntVar(&cfg.ClusterSpawn.CPU, cpuFlag, 1, "CPU count")
	cmd.Flags().IntVar(&cfg.ClusterSpawn.Memory, memoryFlag, 512, "Memory (in MB)")
	cmd.Flags().StringVar(&cfg.ClusterSpawn.ImageRef, imageRefFlag, "", "Image Reference")
	cmd.Flags().StringVar(&cfg.ClusterSpawn.Ports, portsFlag, "", "comma-separated list of ports to expose")
	cmd.Flags().StringSliceVar(&cfg.ClusterSpawn.Env, envFlag, []string{}, "list of env variables to pass to container")
	cmd.Flags().StringVar(&cfg.ClusterSpawn.PolicyFile, policyFileFlag, "", "Path to policy file for this spawn (overrides cluster default)")
}

func AddClusterStopFlags(cmd *cobra.Command, cfg *Config) {
	cmd.Flags().StringVar(&cfg.ClusterStop.ID, idFlag, "", "id of VM to be stopped")
}

func AddClusterLogsFlags(cmd *cobra.Command, cfg *Config) {
	cmd.Flags().StringVar(&cfg.ClusterLogs.ID, idFlag, "", "id of VM for fetching logs")
}

func BindCommandToViper(cmd *cobra.Command) {
	bindFlagsToViper(cmd.PersistentFlags())
	bindFlagsToViper(cmd.Flags())
}

func bindFlagsToViper(fs *pflag.FlagSet) {
	fs.VisitAll(func(flag *pflag.Flag) {
		_ = viper.BindPFlag(flag.Name, flag)
		_ = viper.BindEnv(flag.Name)

		if !flag.Changed && viper.IsSet(flag.Name) {
			val := viper.Get(flag.Name)
			_ = fs.Set(flag.Name, fmt.Sprintf("%v", val))
		}
	})
}
