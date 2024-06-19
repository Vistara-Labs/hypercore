package ports

import (
	"context"
	"vistara-node/pkg/models"
)

type RepositoryGetOptions struct {
	Name      string
	Namespace string
	Version   string
	UID       string
}

// MicroVMRepository is the port definition for a microvm repository.
type MicroVMRepository interface {
	// Save will save the supplied microvm spec.
	Save(ctx context.Context, microvm *models.MicroVM) (*models.MicroVM, error)
	// Delete will delete the supplied microvm.
	Delete(ctx context.Context, options RepositoryGetOptions) error
	// Get will get the microvm spec with the given name/namespace.
	Get(ctx context.Context, options RepositoryGetOptions) (*models.MicroVM, error)
	// GetAll will get a list of microvm details. If namespace is an empty string all
	// details of microvms will be returned.
	GetAll(ctx context.Context) ([]*models.MicroVM, error)
	// Exists checks to see if the microvm spec exists in the repo.
	Exists(ctx context.Context, vmid models.VMID) (bool, error)
	// Returns a container ID
	CreateContainer(ctx context.Context, ref string) (string, error)
	DeleteContainer(ctx context.Context, containerId string) error
}
