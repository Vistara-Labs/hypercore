package hypercore

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"vistara-node/pkg/cluster"

	"google.golang.org/grpc"

	"vistara-node/pkg/containerd"
	"vistara-node/pkg/defaults"
	pb "vistara-node/pkg/proto/cluster"

	"github.com/spf13/cobra"

	"vistara-node/pkg/models"

	"github.com/containerd/containerd/cio"
	"github.com/containerd/typeurl/v2"
	"github.com/google/uuid"
	toml "github.com/pelletier/go-toml/v2"
	"google.golang.org/grpc/credentials/insecure"

	log "github.com/sirupsen/logrus"
)

type HacConfig struct {
	Spacecore struct {
		name        string
		description string
	}
	Hardware struct {
		Cores     int32
		Memory    int32
		Kernel    string
		Drive     string
		Interface string
		Ref       string
	}
}

func containerdConfig(cfg *Config) *containerd.Config {
	return &containerd.Config{
		SocketPath:         cfg.CtrSocketPath,
		ContainerNamespace: cfg.CtrNamespace,
	}
}

func AttachCommand(cfg *Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "attach",
		Short: "attach to a VM",
		Args:  cobra.ExactArgs(1),
		PreRunE: func(c *cobra.Command, _ []string) error {
			BindCommandToViper(c)

			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			repo, err := containerd.NewMicroVMRepository(containerdConfig(cfg))
			if err != nil {
				return err
			}

			return repo.Attach(cmd.Context(), os.Args[2])
		},
	}

	AddCommonFlags(cmd, cfg)

	return cmd
}

func ClusterSpawnCommand(cfg *Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "spawn",
		Short: "spawn a VM in a cluster",
		PreRunE: func(c *cobra.Command, _ []string) error {
			BindCommandToViper(c)

			return nil
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			ports := map[uint32]uint32{}
			for _, portMap := range strings.Split(cfg.ClusterSpawn.Ports, ",") {
				hostToContainer := strings.Split(portMap, ":")
				if len(hostToContainer) != 2 {
					return fmt.Errorf("invalid port mapping: %s", portMap)
				}

				hostPort, err := strconv.Atoi(hostToContainer[0])
				if err != nil {
					return err
				}

				containerPort, err := strconv.Atoi(hostToContainer[1])
				if err != nil {
					return err
				}

				ports[uint32(hostPort)] = uint32(containerPort)
			}

			conn, err := grpc.NewClient(cfg.GrpcBindAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				return err
			}
			defer conn.Close()

			c := pb.NewClusterServiceClient(conn)
			resp, err := c.Spawn(context.Background(), &pb.VmSpawnRequest{
				Cores:    uint32(cfg.ClusterSpawn.CPU),
				Memory:   uint32(cfg.ClusterSpawn.Memory),
				ImageRef: cfg.ClusterSpawn.ImageRef,
				Ports:    ports,
				Env:      cfg.ClusterSpawn.Env,
			})
			if err != nil {
				return err
			}

			log.Infof("Got response: %v", resp)

			return nil
		},
	}

	AddClusterSpawnFlags(cmd, cfg)

	return cmd
}

func ClusterStopCommand(cfg *Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stop",
		Short: "stop a VM in a cluster",
		PreRunE: func(c *cobra.Command, _ []string) error {
			BindCommandToViper(c)

			return nil
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			conn, err := grpc.NewClient(cfg.GrpcBindAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				return err
			}
			defer conn.Close()

			c := pb.NewClusterServiceClient(conn)
			resp, err := c.Stop(context.Background(), &pb.VmStopRequest{
				Id: cfg.ClusterStop.ID,
			})
			if err != nil {
				return err
			}

			log.Infof("Got response: %v", resp)

			return nil
		},
	}

	AddClusterStopFlags(cmd, cfg)

	return cmd
}

func ClusterLogsCommand(cfg *Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logs",
		Short: "get logs of a workload in a cluster",
		PreRunE: func(c *cobra.Command, _ []string) error {
			BindCommandToViper(c)

			return nil
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			conn, err := grpc.NewClient(cfg.GrpcBindAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				return err
			}
			defer conn.Close()

			c := pb.NewClusterServiceClient(conn)
			resp, err := c.Logs(context.Background(), &pb.VmLogsRequest{
				Id: cfg.ClusterLogs.ID,
			})
			if err != nil {
				return err
			}

			log.Infof("Got logs: %s\n", resp.GetLogs())

			return nil
		},
	}

	AddClusterLogsFlags(cmd, cfg)

	return cmd
}

func ClusterListCommand(cfg *Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "list VMs in a cluster",
		PreRunE: func(c *cobra.Command, _ []string) error {
			BindCommandToViper(c)

			return nil
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			conn, err := grpc.NewClient(cfg.GrpcBindAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				return err
			}
			defer conn.Close()

			c := pb.NewClusterServiceClient(conn)
			resp, err := c.List(context.Background(), &pb.VmQueryRequest{})
			if err != nil {
				return err
			}

			log.Infof("Got response: %+v", resp)

			return nil
		},
	}

	return cmd
}

func ClusterMetricsCommand(cfg *Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "metrics",
		Short: "display IBRL metrics for cluster nodes",
		PreRunE: func(c *cobra.Command, _ []string) error {
			BindCommandToViper(c)
			return nil
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			conn, err := grpc.NewClient(cfg.GrpcBindAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				return err
			}
			defer conn.Close()

			c := pb.NewClusterServiceClient(conn)
			resp, err := c.List(context.Background(), &pb.VmQueryRequest{})
			if err != nil {
				return err
			}

			// Display metrics in a formatted table
			log.Info("IBRL Cluster Metrics:")
			log.Info("====================")
			log.Info("")

			for _, nodeState := range resp.GetStates() {
				node := nodeState.GetNode()
				beacon := nodeState.GetBeacon()

				log.Infof("Node: %s", node.GetId())

				if beacon != nil {
					log.Infof("  Beacon ID:       %s", beacon.GetBeaconNodeId())
					log.Infof("  Latency:         %.2f ms", beacon.GetLatencyMs())
					log.Infof("  Jitter:          %.2f ms", beacon.GetJitterMs())
					log.Infof("  Packet Loss:     %.2f%%", beacon.GetPacketLoss())
					log.Infof("  Queue Depth:     %d", beacon.GetQueueDepth())
					log.Infof("  Price/GB:        $%.4f", beacon.GetPricePerGb())
					log.Infof("  Reputation:      %s", beacon.GetReputationScore())
					log.Infof("  Capabilities:    %v", beacon.GetNodeCapabilities())
				} else {
					log.Info("  Beacon: Not available")
				}

				log.Infof("  Workloads:       %d", len(nodeState.GetWorkloads()))
				log.Info("")
			}

			return nil
		},
	}

	return cmd
}

func ClusterCommand(cfg *Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cluster",
		Short: "join a hypercore cluster",
		Args:  cobra.MaximumNArgs(1),
		PreRunE: func(c *cobra.Command, _ []string) error {
			BindCommandToViper(c)

			return nil
		},
		RunE: func(_ *cobra.Command, args []string) error {
			logger := log.New()

			repo, err := containerd.NewMicroVMRepository(containerdConfig(cfg))
			if err != nil {
				// On Mac, containerd is not available - only allow metrics/list commands
				logger.WithError(err).Warn("containerd not available - cluster node mode disabled")
				logger.Info("Use 'hypercore cluster metrics' or 'hypercore cluster list' to query existing clusters")
				return fmt.Errorf("containerd operations not supported on Mac: %w", err)
			}

			var tlsConfig *cluster.TLSConfig

			if cfg.ClusterTLSKey != "" && cfg.ClusterTLSCert != "" {
				tlsConfig = &cluster.TLSConfig{
					CertFile: cfg.ClusterTLSCert,
					KeyFile:  cfg.ClusterTLSKey,
				}
			}

			agent, err := cluster.NewAgent(logger, cfg.ClusterBaseURL, cfg.ClusterBindAddr, cfg.RespawnOnNodeFailure, repo, tlsConfig, cfg.ClusterPolicyFile, cfg.BeaconEndpoint, cfg.BeaconPrice, cfg.BeaconReputation)
			if err != nil {
				return err
			}

			if len(args) > 0 {
				if err := agent.Join(args[0]); err != nil {
					return err
				}
			}

			httpServer, grpcServer := cluster.NewServer(logger, agent)
			grpcListener, err := net.Listen("tcp", cfg.GrpcBindAddr)
			if err != nil {
				return err
			}

			quitWg := sync.WaitGroup{}
			quitWg.Add(3)

			go func() {
				defer quitWg.Done()
				if err := http.ListenAndServe(cfg.HTTPBindAddr, httpServer); err != nil {
					panic(err)
				}
			}()

			go func() {
				defer quitWg.Done()
				if err := grpcServer.Serve(grpcListener); err != nil {
					panic(err)
				}
			}()

			go func() {
				defer quitWg.Done()
				agent.Handler()
			}()

			quitWg.Wait()

			return nil
		},
	}

	cmd.AddCommand(ClusterSpawnCommand(cfg))
	cmd.AddCommand(ClusterStopCommand(cfg))
	cmd.AddCommand(ClusterLogsCommand(cfg))
	cmd.AddCommand(ClusterListCommand(cfg))
	cmd.AddCommand(ClusterMetricsCommand(cfg))

	// TODO remove hac/vmm flags
	AddCommonFlags(cmd, cfg)
	AddClusterFlags(cmd, cfg)

	return cmd
}

func ListCommand(cfg *Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List running VMs",
		PreRunE: func(c *cobra.Command, _ []string) error {
			BindCommandToViper(c)

			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			repo, err := containerd.NewMicroVMRepository(containerdConfig(cfg))
			if err != nil {
				return err
			}

			tasks, err := repo.GetTasks(cmd.Context())
			if err != nil {
				return err
			}

			for _, task := range tasks {
				log.Infof("Task %s, Container %s\n", task.GetID(), task.GetContainerID())
			}

			return nil
		},
	}

	AddCommonFlags(cmd, cfg)

	return cmd
}

func SpawnCommand(cfg *Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "spawn",
		Short: "Spawn a VM under Hypercore",
		PreRunE: func(c *cobra.Command, _ []string) error {
			BindCommandToViper(c)

			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			typeurl.Register(&models.MicroVMSpec{}, "models.MicroVMSpec")

			repo, err := containerd.NewMicroVMRepository(containerdConfig(cfg))
			if err != nil {
				return err
			}

			hacPath, err := filepath.Abs(cfg.HACFile)
			if err != nil {
				return err
			}

			hacContents, err := os.ReadFile(hacPath)
			if err != nil {
				return err
			}

			vmUUID := uuid.NewString()
			hacConfig := HacConfig{}

			if err := toml.Unmarshal(hacContents, &hacConfig); err != nil {
				return err
			}

			log.Infof("Creating VM '%s' with config %+v\n", vmUUID, hacConfig)

			var id string

			switch cfg.DefaultVMProvider {
			case "runc":
				id, err = repo.CreateContainer(cmd.Context(), containerd.CreateContainerOpts{
					ImageRef:    hacConfig.Hardware.Ref,
					Snapshotter: "",
					Runtime: struct {
						Name    string
						Options interface{}
					}{
						Name: "io.containerd.runc.v2",
					},
					CioCreator: cio.NewCreator(cio.WithStdio),
				})
			case "firecracker":
				fallthrough
			case "cloudhypervisor":
				id, err = repo.CreateContainer(cmd.Context(), containerd.CreateContainerOpts{
					ImageRef:    hacConfig.Hardware.Ref,
					Snapshotter: "devmapper",
					Runtime: struct {
						Name    string
						Options interface{}
					}{
						Name: "hypercore.example",
						Options: &models.MicroVMSpec{
							Provider:   cfg.DefaultVMProvider,
							VCPU:       hacConfig.Hardware.Cores,
							MemoryInMb: hacConfig.Hardware.Memory,
							HostNetDev: hacConfig.Hardware.Interface,
							Kernel:     hacConfig.Hardware.Kernel,
							RootfsPath: hacConfig.Hardware.Drive,
						},
					},
					CioCreator: cio.NewCreator(cio.WithFIFODir(defaults.StateRootDir+"/fifo"), cio.WithStreams(&bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{})),
				})
			case "docker":
				client, err := NewDockerClient()
				if err != nil {
					return err
				}

				id, err = client.Start(cmd.Context(), hacConfig.Hardware.Ref)
				if err != nil {
					return err
				}
			}

			if err != nil {
				return err
			}

			log.Infof("ID: %s\n", id)

			return nil
		},
	}

	AddCommonFlags(cmd, cfg)

	return cmd
}

func StopCommand(cfg *Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stop",
		Short: "stop a VM",
		Args:  cobra.ExactArgs(1),
		PreRunE: func(c *cobra.Command, _ []string) error {
			BindCommandToViper(c)

			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			repo, err := containerd.NewMicroVMRepository(containerdConfig(cfg))
			if err != nil {
				return err
			}

			code, err := repo.DeleteContainer(cmd.Context(), os.Args[2])
			if err != nil {
				return err
			}

			os.Exit(int(code))

			return nil
		},
	}

	AddCommonFlags(cmd, cfg)

	return cmd
}
