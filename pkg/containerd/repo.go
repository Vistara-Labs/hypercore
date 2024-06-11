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
	logger.Infof("Saving microvm spec %v", microvm.ID)

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

	namespacesCtx := namespaces.WithNamespace(ctx, r.config.Namespace)

	// follow the rest for saving the microvm spec to containerd repo
	leaseCtx, err := withOwnerLease(namespacesCtx, microvm.ID.String(), r.client)
	if err != nil {
		return nil, fmt.Errorf("getting lease for owner: %w", err)
	}

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
func (r *containerdRepo) Delete(ctx context.Context, options ports.RepositoryGetOptions) error {
    namespaceCtx := namespaces.WithNamespace(ctx, r.config.Namespace)
	spec, err := r.findDigestForSpec(namespaceCtx, options)
	if err != nil || spec == nil {
		return fmt.Errorf("failed to find digest: %w", err)
	}

	if err = r.client.ContentStore().Delete(namespaceCtx, *spec[0]); err != nil {
		return fmt.Errorf("failed to delete from content store: %w", err)
	}

	return nil
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

	spec, err := r.get(ctx, options)

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
	logger.Debugf("Got microvm spec in Get %v", spec.Spec.Provider)

	return spec, nil
}

// GetAll implements ports.MicroVMRepository.
func (r *containerdRepo) GetAll(ctx context.Context) ([]*models.MicroVM, error) {
	namespaceCtx := namespaces.WithNamespace(ctx, r.config.Namespace)
	digests, err := r.findDigestForSpec(namespaceCtx, ports.RepositoryGetOptions{})

	if err != nil {
		return nil, fmt.Errorf("finding content in store: %w", err)
	}

	models := make([]*models.MicroVM, len(digests))
	for idx, digest := range digests {
		model, err := r.getWithDigest(namespaceCtx, digest)
		if err != nil {
			return nil, fmt.Errorf("failed to get digest: %v", digest)
		}

		models[idx] = model
	}

	return models, nil
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
	digests, err := r.findDigestForSpec(namespaceCtx, options)

	if err != nil {
		return nil, fmt.Errorf("finding content in store: %w", err)
	}

	if digests == nil {
		return nil, nil
	}

	return r.getWithDigest(namespaceCtx, digests[0])
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
) ([]*digest.Digest, error) {
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

	allFilters := strings.Join(combinedFilters, ",")
	store := r.client.ContentStore()

	type digestsMapVal struct {
		version int
		digest  *digest.Digest
	}

	digestsMap := make(map[string]digestsMapVal, 0)

	err := store.Walk(
		ctx,
		func(info content.Info) error {
			version, err := strconv.Atoi(info.Labels[VersionLabel()])

			if err != nil {
				return fmt.Errorf("parsing version number: %w", err)
			}

			name := info.Labels[NameLabel()]

			dgMapVal, ok := digestsMap[name]
			if !ok {
				digestsMap[name] = digestsMapVal{
					version: 0,
					digest:  &info.Digest,
				}
			} else if version > dgMapVal.version {
				digestsMap[name] = digestsMapVal{
					version: version,
					digest:  &info.Digest,
				}
			}

			return nil
		},
		allFilters,
	)
	if err != nil {
		return nil, fmt.Errorf("walking content store for %s: %w", options.Name, err)
	}

	digests := make([]*digest.Digest, 0)

	for _, dgMapVal := range digestsMap {
		digests = append(digests, dgMapVal.digest)
	}

	// Single digest requested but none found
	if len(digests) == 0 {
		return nil, nil
	}

	return digests, nil
}
