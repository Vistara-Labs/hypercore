//go:build wireinject
// +build wireinject

package inject

import (
	"vistara-node/internal/config"
	"vistara-node/pkg/app"
	"vistara-node/pkg/containerd"
	"vistara-node/pkg/defaults"
	"vistara-node/pkg/ports"
	"vistara-node/pkg/ulid"

	"vistara-node/pkg/api/grpc"

	"github.com/google/wire"
	"github.com/spf13/afero"
)

// wire: /Users/mayurchougule/development/vistara/vistara-node/internal/inject/wire.go:15:1:
//
//	inject InitializePorts: no provider found for *vistara-node/pkg/ports.Collection, output of injector
//
// wire: error loading packages
func InitializePorts(cfg *config.Config) (*ports.Collection, error) {
	wire.Build(
		containerd.NewMicroVMRepository,
		ulid.New,
		// hypervisor.NewFromConfig,
		appPorts,
		appConfig,
		containerdConfig,
		afero.NewOsFs,
	)
	return nil, nil
}

func InitializeApp(cfg *config.Config, ports *ports.Collection) app.App {
	wire.Build(app.New, appConfig)

	return nil
}

func InializeController(app application.App, ports *ports.Collection) *controllers.MicroVMController {
	wire.Build(controllers.New, eventSvcFromScope, reconcileUCFromApp, queryUCFromApp)

	return nil
}

func InitializeGRPCServer(cfg *config.Config, app app.App, ports *ports.Collection) *grpc.Server {
	wire.Build(grpc.NewServer, commandSvcFromApp)

	return nil
}

func appConfig(cfg *config.Config) *app.Config {
	return &app.Config{
		RootStateDir:    cfg.StateRootDir,
		MaximumRetry:    3,
		DefaultProvider: cfg.DefaultVMProvider,
	}
}

func commandSvcFromApp(app application.App) *ports.MicroVMService {
	return app
}

// func appPorts(repo ports.MicroVMRepository, providers map[string]ports.MicroVMService, es ports.EventService, is ports.IDService, ns ports.NetworkService, ims ports.ImageService, fs afero.Fs, ds ports.DiskService) *ports.Collection {
func appPorts(repo ports.MicroVMRepository, providers map[string]ports.MicroVMService) *ports.Collection {
	return &ports.Collection{
		Repo:             repo,
		MicrovmProviders: providers,
		// EventService:      es,
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

// func networkConfig(cfg *config.Config) *network.Config {
// 	return &network.Config{
// 		ParentDeviceName: cfg.ParentIface,
// 		BridgeName:       cfg.BridgeName,
// 	}
// }

// func InitializeGRPCServer(app application.App) ports.MicroVMGRPCService {
// 	wire.Build(microvmgrpc.NewServer, queryUCFromApp, commandUCFromApp)

// 	return nil
// }

// func containerdConfig(cfg *config.Config) *containerd.Config {
// 	return &containerd.Config{
// 		SnapshotterKernel: cfg.CtrSnapshotterKernel,
// 		SnapshotterVolume: defaults.ContainerdVolumeSnapshotter,
// 		SocketPath:        cfg.CtrSocketPath,
// 		Namespace:         cfg.CtrNamespace,
// 	}
// }

// func networkConfig(cfg *config.Config) *network.Config {
// 	return &network.Config{
// 		ParentDeviceName: cfg.ParentIface,
// 		BridgeName:       cfg.BridgeName,
// 	}
// }

// func eventSvcFromScope(ports *ports.Collection) ports.EventService {
// 	return ports.EventService
// }

// func reconcileUCFromApp(app application.App) ports.ReconcileMicroVMsUseCase {
// 	return app
// }

// func queryUCFromApp(app application.App) ports.MicroVMQueryUseCases {
// 	return app
// }

// func commandUCFromApp(app application.App) ports.MicroVMCommandUseCases {
// 	return app
// }
