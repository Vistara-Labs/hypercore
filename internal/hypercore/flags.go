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
	hacFileFlag          = "hac"
	containerdSocketFlag = "containerd-socket"
	containerdNamespace  = "containerd-ns"
	vmProviderFlag       = "provider"
	grpcBindAddrFlag     = "grpc-bind-addr"
	clusterBindAddrFlag  = "cluster-bind-addr"
	cpuFlag              = "cpu"
	memoryFlag           = "mem"
	imageRefFlag         = "image-ref"
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
	cmd.Flags().StringVar(&cfg.ClusterBindAddr, clusterBindAddrFlag, ":7946", "Cluster bind address")
}

func AddClusterSpawnFlags(cmd *cobra.Command, cfg *Config) {
	cmd.Flags().StringVar(&cfg.GrpcBindAddr, grpcBindAddrFlag, "0.0.0.0:8000", "GRPC Server bind address")
	cmd.Flags().IntVar(&cfg.ClusterSpawn.CPU, cpuFlag, 1, "CPU count")
	cmd.Flags().IntVar(&cfg.ClusterSpawn.Memory, memoryFlag, 512, "Memory (in MB)")
	cmd.Flags().StringVar(&cfg.ClusterSpawn.ImageRef, imageRefFlag, "", "Image Reference")
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
