package hypercore

import (
	"bytes"
	"net"
	"os"
	"path/filepath"
	"sync"
	"vistara-node/pkg/cluster"

	"vistara-node/pkg/containerd"
	"vistara-node/pkg/defaults"

	"github.com/spf13/cobra"

	"vistara-node/pkg/models"

	"github.com/containerd/containerd/cio"
	"github.com/containerd/typeurl/v2"
	"github.com/google/uuid"
	toml "github.com/pelletier/go-toml/v2"

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

func ClusterCommand(cfg *Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cluster",
		Short: "join a hypercore cluster",
		Args:  cobra.MaximumNArgs(1),
		PreRunE: func(c *cobra.Command, _ []string) error {
			BindCommandToViper(c)

			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			logger := log.New()

			repo, err := containerd.NewMicroVMRepository(containerdConfig(cfg))
			if err != nil {
				return err
			}

			agent, err := cluster.NewAgent(cfg.ClusterBindAddr, repo, logger)

			if len(os.Args) > 2 {
				if err := agent.Join(os.Args[2]); err != nil {
					return err
				}
			}

			grpcServer := cluster.NewServer(logger, agent)
			grpcListener, err := net.Listen("tcp", cfg.GrpcBindAddr)
			if err != nil {
				return err
			}

			quitWg := sync.WaitGroup{}
			quitWg.Add(2)

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
					Snapshotter: "blockfile",
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
