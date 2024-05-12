package flags

import (
	"fmt"
	"vistara-node/internal/config"
	"vistara-node/pkg/defaults"
	"vistara-node/pkg/hypervisor/firecracker"

	"github.com/spf13/cobra"
)

const (
	grpcEndpointFlag          = "grpc-endpoint"
	httpEndpointFlag          = "http-endpoint"
	parentIfaceFlag           = "parent-iface"
	bridgeNameFlag            = "bridge-name"
	disableReconcileFlag      = "disable-reconcile"
	disableAPIFlag            = "disable-api"
	firecrackerBinFlag        = "firecracker-bin"
	firecrackerDetachFlag     = "firecracker-detach"
	containerdSocketFlag      = "containerd-socket"
	kernelSnapshotterFlag     = "containerd-kernel-ss"
	containerdNamespace       = "containerd-ns"
	maximumRetryFlag          = "maximum-retry"
	basicAuthTokenFlag        = "basic-auth-token"
	insecureFlag              = "insecure"
	tlsCertFlag               = "tls-cert"
	tlsKeyFlag                = "tls-key"
	tlsClientValidateFlag     = "tls-client-validate"
	tlsClientCAFlag           = "tls-client-ca"
	debugEndpointFlag         = "debug-endpoint"
	cloudHypervisorBinFlag    = "cloudhypervisor-bin"
	cloudHypervisorDetachFlag = "cloudhypervisor-detach"
)

// AddGRPCServerFlagsToCommand will add gRPC server flags to the supplied command.
func AddGRPCServerFlagsToCommand(cmd *cobra.Command, cfg *config.Config) {
	cmd.Flags().StringVar(&cfg.GRPCAPIEndpoint,
		grpcEndpointFlag,
		defaults.GRPCAPIEndpoint,
		"The endpoint for the gRPC server to listen on.")

	cmd.Flags().StringVar(&cfg.StateRootDir,
		"state-dir",
		defaults.StateRootDir,
		"The directory to use for the as the root for runtime state.")

	cmd.Flags().DurationVar(&cfg.ResyncPeriod,
		"resync-period",
		defaults.ResyncPeriod,
		"Reconcile the specs to resynchronise them based on this period.")

	cmd.Flags().DurationVar(&cfg.DeleteVMTimeout,
		"deleteMicroVM-timeout",
		defaults.DeleteVMTimeout,
		"The timeout for deleting a microvm.")
}

// AddGWServerFlagsToCommand will add gRPC HTTP gateway flags to the supplied command.
func AddGWServerFlagsToCommand(cmd *cobra.Command, cfg *config.Config) {
	cmd.Flags().BoolVar(&cfg.EnableHTTPGateway,
		"enable-http",
		false,
		"Should the API be exposed via HTTP.")

	cmd.Flags().StringVar(&cfg.HTTPAPIEndpoint,
		httpEndpointFlag,
		defaults.HTTPAPIEndpoint,
		"The endpoint for the HTTP proxy to the gRPC service to listen on.")
}

// AddNetworkFlagsToCommand will add various network flags to the command.
func AddNetworkFlagsToCommand(cmd *cobra.Command, cfg *config.Config) error {
	cmd.Flags().StringVar(&cfg.ParentIface,
		parentIfaceFlag,
		"",
		"The parent iface for the network interfaces. Note it could also be a bond")

	cmd.Flags().StringVar(
		&cfg.BridgeName,
		bridgeNameFlag,
		"",
		"The name of the Linux bridge to attach tap devices to by default")

	return nil
}

// AddHiddenFlagsToCommand will add hidden flags to the supplied command.
func AddHiddenFlagsToCommand(cmd *cobra.Command, cfg *config.Config) error {
	cmd.Flags().BoolVar(&cfg.DisableReconcile,
		disableReconcileFlag,
		false,
		"Set to true to stop the reconciler running")

	cmd.Flags().IntVar(&cfg.MaximumRetry,
		maximumRetryFlag,
		defaults.MaximumRetry,
		"Number of times to retry failed reconciliation")

	cmd.Flags().BoolVar(&cfg.DisableAPI,
		disableAPIFlag,
		false,
		"Set to true to stop the api server running")

	if err := cmd.Flags().MarkHidden(disableReconcileFlag); err != nil {
		return fmt.Errorf("setting %s as hidden: %w", disableReconcileFlag, err)
	}

	if err := cmd.Flags().MarkHidden(maximumRetryFlag); err != nil {
		return fmt.Errorf("setting %s as hidden: %w", maximumRetryFlag, err)
	}

	if err := cmd.Flags().MarkHidden(disableAPIFlag); err != nil {
		return fmt.Errorf("setting %s as hidden: %w", disableAPIFlag, err)
	}

	return nil
}

// AddMicrovmProviderFlagsToCommand will add the microvm provider flags to the supplied command
func AddMicrovmProviderFlagsToCommand(cmd *cobra.Command, cfg *config.Config) {
	addFirecrackerFlagsToCommand(cmd, cfg)
	// addCloudHypervisorFlagsToCommand(cmd, cfg)
	cmd.Flags().StringVar(&cfg.DefaultVMProvider, "default-provider",
		firecracker.HypervisorName, "The name of the vm provider to use by default if not supplied in the create request.")
}

// AddContainerDFlagsToCommand will add the containerd specific flags to the supplied cobra command.
func AddContainerDFlagsToCommand(cmd *cobra.Command, cfg *config.Config) {
	cmd.Flags().StringVar(&cfg.CtrSocketPath,
		containerdSocketFlag,
		defaults.ContainerdSocket,
		"The path to the containerd socket.")

	cmd.Flags().StringVar(&cfg.CtrSnapshotterKernel,
		kernelSnapshotterFlag,
		defaults.ContainerdKernelSnapshotter,
		"The name of the snapshotter to use with containerd for kernel/initrd images.")

	cmd.Flags().StringVar(&cfg.CtrNamespace,
		containerdNamespace,
		defaults.ContainerdNamespace,
		"The name of the containerd namespace to use.")
}

func AddDebugFlagsToCommand(cmd *cobra.Command, cfg *config.Config) {
	cmd.Flags().StringVar(&cfg.DebugEndpoint,
		debugEndpointFlag,
		"",
		"The endpoint for the debug web server to listen on. It must include a port (e.g. localhost:10500).  An empty string means disable the debug endpoint.")
}

func addFirecrackerFlagsToCommand(cmd *cobra.Command, cfg *config.Config) {
	cmd.Flags().StringVar(&cfg.FirecrackerBin,
		firecrackerBinFlag,
		defaults.FirecrackerBin,
		"The path to the firecracker binary to use.")
	cmd.Flags().BoolVar(&cfg.FirecrackerDetatch,
		firecrackerDetachFlag,
		defaults.FirecrackerDetach,
		"If true the child firecracker processes will be detached from the parent hypercore process.")
}

func addCloudHypervisorFlagsToCommand(cmd *cobra.Command, cfg *config.Config) error {
	cmd.Flags().StringVar(&cfg.CloudHypervisorBin,
		cloudHypervisorBinFlag,
		defaults.CloudHypervisorBin,
		"The path to the cloud hypervisor binary to use.")
	cmd.Flags().BoolVar(&cfg.CloudHypervisorDetatch,
		cloudHypervisorDetachFlag,
		defaults.CloudHypervisorDetach,
		"If true the child cloud hypervisor processes will be detached from the parent hypercore process.")

	return nil
}
