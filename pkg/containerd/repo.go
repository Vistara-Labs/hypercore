package containerd

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"vistara-node/pkg/errors"
	"vistara-node/pkg/log"
	"vistara-node/pkg/models"
	"vistara-node/pkg/ports"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/namespaces"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"

	// v12 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/opencontainers/go-digest"
)

// create a new containerd backed microvm repo
func NewMicroVMRepository(cfg *Config) (ports.MicroVMRepository, error) {
	client, err := containerd.New(cfg.SocketPath)
	fmt.Printf("client: %v\n", client)
	fmt.Printf("cfg.SocketPath: %v\n", cfg.SocketPath)
	if err != nil {
		return nil, fmt.Errorf("creating containerd client: %w", err)
	}
	return NewVMRepoWithClient(cfg, client), nil
}

func NewVMRepoWithClient(cfg *Config, client *containerd.Client) ports.MicroVMRepository {
	return &containerdRepo{
		client: client,
		config: cfg,
		locks:  make(map[string]*sync.RWMutex),
	}
}

type containerdRepo struct {
	client  *containerd.Client
	config  *Config
	locks   map[string]*sync.RWMutex
	locksMu sync.Mutex
}

// Saves the microvm spec to containerd and returns the microvm
func (r *containerdRepo) Save(ctx context.Context, microvm *models.MicroVM) (*models.MicroVM, error) {
	logger := log.GetLogger(ctx).WithField("repo", "containerd_vm")
	logger.Info("Saving microvm spec %v", microvm)

	mu := r.getMutex(microvm.ID.String())
	mu.Lock()
	defer mu.Unlock()

	existingSpec, err := r.get(ctx, ports.RepositoryGetOptions{
		UID:       microvm.ID.UID(),
		Namespace: microvm.ID.Namespace(),
		Name:      microvm.ID.Name(),
	})

	if err != nil {
		return nil, fmt.Errorf("getting existing microvm: %w", err)
	}
	if existingSpec != nil {
		return nil, fmt.Errorf("microvm already exists")
	}

	// logger.Info(microvm.Spec)
	// logger.Info(microvm.ID)
	// logger.Infof("r config namespace %v", r.config.Namespace)
	// namespacesCtx := namespaces.WithNamespace(ctx, microvm.ID.Namespace())
	namespacesCtx := namespaces.WithNamespace(ctx, r.config.Namespace)

	// follow the rest for saving the microvm spec to containerd repo
	leaseCtx, err := withOwnerLease(namespacesCtx, microvm.ID.String(), r.client)
	if err != nil {
		return nil, fmt.Errorf("getting lease for owner: %w", err)
	}

	// logger.Info("Lease context: %v", leaseCtx)

	store := r.client.ContentStore()

	microvm.Version++
	refName := contentRefName(microvm)
	// create a new content ref

	writer, err := store.Writer(leaseCtx, content.WithRef(refName))
	if err != nil {
		return nil, fmt.Errorf("creating content writer: %w", err)
	}
	// logger.Infof("refName: %v %v", refName, content.WithRef(refName))

	// marshal the microvm spec to json
	data, err := json.Marshal(microvm)
	if err != nil {
		return nil, fmt.Errorf("marshalling microvm to json: %w", err)
	}

	// write the microvm spec to the content store
	_, err = writer.Write(data)
	if err != nil {
		return nil, fmt.Errorf("writing microvm to content store: %w", err)
	}

	labels := getVMLabels(microvm)

	// commit the content to the store
	err = writer.Commit(namespacesCtx, 0, "", content.WithLabels(labels))
	if err != nil {
		return nil, fmt.Errorf("committing microvm to content store: %w", err)
	}

	return microvm, nil
}

// Delete implements ports.MicroVMRepository.
func (*containerdRepo) Delete(ctx context.Context, microvm *models.MicroVM) error {
	panic("unimplemented")
}

// Exists implements ports.MicroVMRepository.
func (*containerdRepo) Exists(ctx context.Context, vmid models.VMID) (bool, error) {
	panic("unimplemented")
}

// Get implements ports.MicroVMRepository.
// Get will get the microvm spec with the given name/namespace from the containerd content store.
// If version is not empty, returns with the specified version of the spec.
func (r *containerdRepo) Get(ctx context.Context, options ports.RepositoryGetOptions) (*models.MicroVM, error) {
	logger := log.GetLogger(ctx).WithField("repo", "containerd_vm")
	mu := r.getMutex(options.Name)
	mu.RLock()
	defer mu.RUnlock()
	logger.Infof("Getting microvm spec %v", options)

	spec, err := r.get(ctx, options)
	logger.Infof("Got microvm spec before %v, %w\n", spec, err)

	if err != nil {
		return nil, fmt.Errorf("getting vm spec from store: %w", err)
	}

	if spec == nil {
		return nil, errors.NewSpecNotFound( //nolint: wrapcheck // No need to wrap this error
			options.Name,
			options.Namespace,
			options.Version,
			options.UID)
	}
	logger.Infof("Got microvm spec in Get %v", spec)

	return spec, nil
}

// GetAll implements ports.MicroVMRepository.
func (*containerdRepo) GetAll(ctx context.Context, query models.ListMicroVMQuery) ([]*models.MicroVM, error) {
	panic("unimplemented")
}

func (c *containerdRepo) Create() (string, error) {
	return "", nil
}

func getVMLabels(microvm *models.MicroVM) map[string]string {
	labels := map[string]string{
		NameLabel():      microvm.ID.Name(),
		NamespaceLabel(): microvm.ID.Namespace(),
		TypeLabel():      MicroVMSpecType,
		VersionLabel():   strconv.Itoa(microvm.Version),
		UIDLabel():       microvm.ID.UID(),
	}
	fmt.Printf("in getvmlabels labels: %v\n", labels)

	return labels
}

func (r *containerdRepo) getMutex(name string) *sync.RWMutex {
	r.locksMu.Lock()
	defer r.locksMu.Unlock()

	namedMu, ok := r.locks[name]
	if ok {
		return namedMu
	}

	mu := &sync.RWMutex{}
	r.locks[name] = mu

	return mu
}

func (r *containerdRepo) get(ctx context.Context, options ports.RepositoryGetOptions) (*models.MicroVM, error) {
	namespaceCtx := namespaces.WithNamespace(ctx, r.config.Namespace)

	digest, err := r.findDigestForSpec(namespaceCtx, options)
	fmt.Printf("\ndigest: %v\n", digest)
	if err != nil {
		return nil, fmt.Errorf("finding content in store: %w", err)
	}

	if digest == nil {
		return nil, nil
	}

	return r.getWithDigest(namespaceCtx, digest)
}

func (r *containerdRepo) findAllDigestForSpec(ctx context.Context, name, namespace string) ([]*digest.Digest, error) {
	store := r.client.ContentStore()
	idLabelFilter := labelFilter(NameLabel(), name)
	nsLabelFilter := labelFilter(NamespaceLabel(), namespace)
	combinedFilters := []string{idLabelFilter, nsLabelFilter}
	allFilters := strings.Join(combinedFilters, ",")
	digests := []*digest.Digest{}

	err := store.Walk(
		ctx,
		func(i content.Info) error {
			digests = append(digests, &i.Digest)
			fmt.Printf("digests: %v\n", digests)

			return nil
		},
		allFilters,
	)
	if err != nil {
		return nil, fmt.Errorf("walking content store for %s: %w", name, err)
	}

	return digests, nil
}

func (r *containerdRepo) getWithDigest(ctx context.Context, metadigest *digest.Digest) (*models.MicroVM, error) {
	readData, err := content.ReadBlob(ctx, r.client.ContentStore(), v1.Descriptor{
		Digest: *metadigest,
	})
	if err != nil {
		return nil, fmt.Errorf("reading content %s: %w", metadigest, "ErrReadingContent")
	}

	microvm := &models.MicroVM{}

	err = json.Unmarshal(readData, microvm)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling json content to microvm: %w", err)
	}

	return microvm, nil
}

func (r *containerdRepo) findDigestForSpec(ctx context.Context,
	options ports.RepositoryGetOptions,
) (*digest.Digest, error) {
	var digest *digest.Digest

	combinedFilters := []string{}

	if options.Name != "" {
		combinedFilters = append(combinedFilters, labelFilter(NameLabel(), options.Name))
	}

	if options.Namespace != "" {
		combinedFilters = append(combinedFilters, labelFilter(NamespaceLabel(), options.Namespace))
	}

	if options.UID != "" {
		combinedFilters = append(combinedFilters, labelFilter(UIDLabel(), options.UID))
	}

	// if options.Version != "" {
	// 	combinedFilters = append(combinedFilters, labelFilter(VersionLabel(), options.Version))
	// }

	allFilters := strings.Join(combinedFilters, ",")
	store := r.client.ContentStore()
	highestVersion := 0
	fmt.Printf("store: %v\n", store)

	fmt.Printf("allFilters: %v, %v\n", allFilters, options)
	err := store.Walk(
		ctx,
		func(info content.Info) error {
			version, err := strconv.Atoi(info.Labels[VersionLabel()])
			fmt.Printf("version in finddigestforspec : %v, %w\n", version, err)
			if err != nil {
				return fmt.Errorf("parsing version number: %w", err)
			}

			if version > highestVersion {
				digest = &info.Digest
				highestVersion = version
			}

			return nil
		},
		allFilters,
	)
	fmt.Printf("digest in finddigestforspec : %v, %w\n", digest, err)
	if err != nil {
		return nil, fmt.Errorf("walking content store for %s: %w", options.Name, err)
	}

	return digest, nil
}
