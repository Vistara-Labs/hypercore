package ports

import "time"

type Collection struct {
	Repo              MicroVMRepository
	MicrovmProviders  map[string]MicroVMService
	IdentifierService IDService
	EventService      EventService
	// NetworkService    NetworkService
	// ImageService      ImageService
	// FileSystem        afero.Fs
	Clock func() time.Time
}
