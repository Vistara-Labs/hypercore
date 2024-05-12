package run

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	cmdflags "vistara-node/internal/command/flags"
	"vistara-node/internal/config"
	"vistara-node/internal/inject"

	grpcapi "vistara-node/pkg/api"
	vm1 "vistara-node/pkg/api/services/microvm"
	"vistara-node/pkg/processors"

	"vistara-node/pkg/app"
	"vistara-node/pkg/containerd"
	"vistara-node/pkg/defaults"
	"vistara-node/pkg/flags"
	"vistara-node/pkg/godisk"
	"vistara-node/pkg/hypervisor"
	"vistara-node/pkg/log"
	"vistara-node/pkg/network"
	"vistara-node/pkg/ports"

	grpc_mw "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func NewCommand(cfg *config.Config) (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run the Vistara node",
		PreRunE: func(c *cobra.Command, _ []string) error {
			flags.BindCommandToViper(c)

			logger := log.GetLogger(c.Context())
			logger.Infof("Starting Vistara node")

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd.Context(), cfg)
		},
	}

	cmdflags.AddGRPCServerFlagsToCommand(cmd, cfg)
	cmdflags.AddContainerDFlagsToCommand(cmd, cfg)
	cmdflags.AddMicrovmProviderFlagsToCommand(cmd, cfg)
	cmdflags.AddDebugFlagsToCommand(cmd, cfg)
	cmdflags.AddGWServerFlagsToCommand(cmd, cfg)

	return cmd, nil
}

func serve(ctx context.Context, cfg *config.Config) error {
	logger := log.GetLogger(ctx)
	logger.Info("Starting hypercore gRPC server")

	// Create a context that will be canceled when the user sends a SIGINT
	ports, err := InitializePorts(cfg)
	logger.Infof("Initialized ports %v", ports)
	if err != nil {
		return err
	}
	app := inject.InitializeApp(cfg, ports)
	println(app)
	logger.Infof("Initialized app %v", app)

	// initialize gRPC server with commandSvc an instance of ports.MicroVMService
	vmGRPCService := grpcapi.NewServer(app)
	logger.Infof("Initialized gRPC server %v", vmGRPCService)

	serverOpts, _ := generateOpts(ctx, cfg)

	// start gRPC server
	grpcServer := grpc.NewServer(serverOpts...)

	vm1.RegisterVMServiceServer(grpcServer, vmGRPCService)
	grpc_prometheus.Register(grpcServer)
	http.Handle("/metrics", promhttp.Handler())

	go func() {
		<-ctx.Done()
		logger.Infof("Shutting down gRPC server")
		grpcServer.GracefulStop()
	}()

	logger.Infof("Starting gRPC server on %s", cfg.GRPCAPIEndpoint)

	listener, err := net.Listen("tcp", cfg.GRPCAPIEndpoint)
	if err != nil {
		return fmt.Errorf("failed to listen: %v", err)
	}
	defer listener.Close()

	reflection.Register(grpcServer)

	if err := grpcServer.Serve(listener); err != nil {
		return fmt.Errorf("failed to serve: %v", err)
	}

	return nil
}

func run(ctx context.Context, cfg *config.Config) error {
	logger := log.GetLogger(ctx)
	logger.Info("Starting vistara hypercore")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	wg := &sync.WaitGroup{}
	ctx, cancel := context.WithCancel(log.WithLogger(ctx, logger))

	if !cfg.DisableAPI {
		wg.Add(1)

		go func() {
			defer wg.Done()

			if err := serve(ctx, cfg); err != nil {
				logger.Errorf("failed to start gRPC server %v", err)
				// cancel all processes if at least one fails
				cancel()
			}
		}()
	}

	if !cfg.DisableReconcile {
		wg.Add(1)

		go func() {
			defer wg.Done()

			if err := runProcessors(ctx, cfg); err != nil {
				logger.Errorf("failed to run VM processors: %v", err)
				cancel()
			}
		}()
	}
	<-sigChan
	logger.Debug("Shutdown signal received, waiting for work to finish")

	cancel()
	wg.Wait()

	logger.Info("Finished all tasks, exiting")

	return nil
}

func commandUCFromApp(app app.App) ports.MicroVMService {
	return app
}

func eventSvcFromScope(p2 *ports.Collection) ports.EventService {
	return p2.EventService
}

func runProcessors(ctx context.Context, cfg *config.Config) error {
	logger := log.GetLogger(ctx)

	// Create a context that will be canceled when the user sends a SIGINT
	ports, err := InitializePorts(cfg)
	logger.Info("Initialized ports %v", ports)
	if err != nil {
		return err
	}
	app := inject.InitializeApp(cfg, ports)

	cmdSvc := commandUCFromApp(app)
	evtSvc := eventSvcFromScope(ports)

	vmProcessors := processors.NewVMProcessor(cmdSvc, evtSvc)

	// Run VM Processor that listens to events
	// puts new events in a queue, r.queue.Enqueue
	// processQueue will process the queue items
	if err := vmProcessors.Run(ctx, 1); err != nil {
		logger.Fatalf("Starting VM processor: %v", err)
	}

	return nil
}

func InitializePorts(cfg *config.Config) (*ports.Collection, error) {
	config2 := containerdConfig(cfg)

	microVMRepository, err := containerd.NewMicroVMRepository(config2)
	fmt.Printf("config2: %v\n", config2)
	fmt.Printf("microVMRepository: %v\n\n", microVMRepository)
	if err != nil {
		return nil, err
	}
	config3 := networkConfig(cfg)
	networkService := network.New(config3)
	fs := afero.NewOsFs()
	diskService := godisk.New(fs)

	v, err := hypervisor.NewFromConfig(cfg, networkService, diskService, fs)

	// v, err := microvm.NewFromConfig(cfg, networkService, diskService, fs)
	if err != nil {
		return nil, err
	}
	eventService, err := containerd.NewEventService(config2)
	if err != nil {
		return nil, err
	}
	// idService := ulid.New()
	// imageService, err := containerd.NewImageService(config2)
	// if err != nil {
	// 	return nil, err
	// }
	// collection := appPorts(microVMRepository, v, eventService, idService, networkService, imageService, fs, diskService)
	collection := appPorts(microVMRepository, v, eventService)

	return collection, nil
}

func appConfig(cfg *config.Config) *app.Config {
	return &app.Config{
		RootStateDir:    cfg.StateRootDir,
		MaximumRetry:    3,
		DefaultProvider: cfg.DefaultVMProvider,
	}
}

func appPorts(
	repo ports.MicroVMRepository,
	providers map[string]ports.MicroVMService,
	es ports.EventService,
) *ports.Collection {

	// , es ports.EventService, is ports.IDService, ns ports.NetworkService, ims ports.ImageService, fs afero.Fs, ds ports.DiskService) *ports.Collection {

	return &ports.Collection{
		Repo:             repo,
		MicrovmProviders: providers,
		EventService:     es,
		// IdentifierService: is,
		// NetworkService:    ns,
		// ImageService:      ims,
		// FileSystem:        fs,
		// Clock:             time.Now,
	}
}

func containerdConfig(cfg *config.Config) *containerd.Config {
	return &containerd.Config{
		SnapshotterKernel: cfg.CtrSnapshotterKernel,
		SnapshotterVolume: defaults.ContainerdVolumeSnapshotter,
		SocketPath:        cfg.CtrSocketPath,
		Namespace:         cfg.CtrNamespace,
	}
}

func networkConfig(cfg *config.Config) *network.Config {
	return &network.Config{
		ParentDeviceName: cfg.ParentIface,
		BridgeName:       cfg.BridgeName,
	}
}

func generateOpts(ctx context.Context, cfg *config.Config) ([]grpc.ServerOption, error) {
	logger := log.GetLogger(ctx)

	opts := []grpc.ServerOption{
		grpc.StreamInterceptor(grpc_prometheus.StreamServerInterceptor),
		grpc.UnaryInterceptor(grpc_prometheus.UnaryServerInterceptor),
	}

	if cfg.BasicAuthToken != "" {
		logger.Info("basic authentication is enabled")

		opts = []grpc.ServerOption{
			grpc.StreamInterceptor(grpc_mw.ChainStreamServer(
				grpc_prometheus.StreamServerInterceptor,
				// grpc_auth.StreamServerInterceptor,
			)),
			grpc.UnaryInterceptor(grpc_mw.ChainUnaryServer(
				grpc_prometheus.UnaryServerInterceptor,
			)),
		}
	} else {
		logger.Warn("basic authentication is DISABLED")
	}

	if !cfg.TLS.Insecure {
		logger.Info("TLS is enabled")

		// creds, err := auth.LoadTLSForGRPC(&cfg.TLS)
		// if err != nil {
		// 	return nil, err
		// }

		// opts = append(opts, grpc.Creds(creds))
	} else {
		logger.Warn("TLS is DISABLED")
	}

	return opts, nil
}
