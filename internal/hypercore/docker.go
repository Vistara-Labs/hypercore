package hypercore

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
)

type DockerClient struct {
	*client.Client
}

func NewDockerClient() (DockerClient, error) {
	client, err := client.NewClientWithOpts()
	if err != nil {
		return DockerClient{}, err
	}

	client.NegotiateAPIVersion(context.Background())

	return DockerClient{client}, nil
}

func (c DockerClient) Start(ctx context.Context, imageRef string) (string, error) {
	readCloser, err := c.ImagePull(ctx, imageRef, image.PullOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to pull image: %w", err)
	}

	readCloser.Close()

	containerResp, err := c.ContainerCreate(
		ctx,
		&container.Config{
			Image: imageRef,
		},
		&container.HostConfig{
			AutoRemove: true,
		},
		nil,
		nil,
		"",
	)
	if err != nil {
		return "", fmt.Errorf("failed to create container: %w", err)
	}

	if err = c.ContainerStart(ctx, containerResp.ID, container.StartOptions{}); err != nil {
		return "", fmt.Errorf("failed to start container: %w", err)
	}

	return "", nil
}

func (c DockerClient) Stop(ctx context.Context, containerID string) error {
	timeout := 15
	err := c.ContainerStop(ctx, containerID, container.StopOptions{Timeout: &timeout})

	if err != nil {
		return fmt.Errorf("failed to remove container %s: %w", containerID, err)
	}

	return nil
}
