package errors

import (
	"errors"
	"fmt"
)

var (
	ErrSpecRequired                       = errors.New("microvm spec is required")
	ErrVMIDRequired                       = errors.New("id for microvm is required")
	ErrNameRequired                       = errors.New("name is required")
	ErrUIDRequired                        = errors.New("uid is required")
	ErrNamespaceRequired                  = errors.New("namespace is required")
	ErrKernelImageRequired                = errors.New("kernel image is required")
	ErrVolumeRequired                     = errors.New("no volumes specified, at least 1 volume is required")
	ErrRootVolumeRequired                 = errors.New("a root volume is required")
	ErrNoMount                            = errors.New("no image mount point")
	ErrNoVolumeMount                      = errors.New("no volume mount point")
	ErrParentIfaceRequiredForMacvtap      = errors.New("a parent network device name is required for macvtap interfaces")
	ErrParentIfaceRequiredForAttachingTap = errors.New("a parent network device name is required for attaching a TAP interface")
	ErrGuestDeviceNameRequired            = errors.New("a guest device name is required")
	ErrUnsupportedIfaceType               = errors.New("unsupported network interface type")
	ErrIfaceNotFound                      = errors.New("network interface not found")
	ErrMissingStatusInfo                  = errors.New("status is not defined")
	ErrUnableToBoot                       = errors.New("microvm is unable to boot")
)

type IncorrectVMIDFormatError struct {
	ActualID string
}

// Error returns the error message.
func (e IncorrectVMIDFormatError) Error() string {
	return fmt.Sprintf("unexpected vmid format: %s", e.ActualID)
}

type specNotFoundError struct {
	name      string
	namespace string
	version   string
	uid       string
}

// Error returns the error message.
func (e specNotFoundError) Error() string {
	if e.version == "" {
		return fmt.Sprintf("microvm spec %s/%s/%s not found", e.namespace, e.name, e.uid)
	}

	return fmt.Sprintf("microvm spec %s/%s/%s not found with version %s", e.namespace, e.name, e.uid, e.version)
}

func NewSpecNotFound(name, namespace, version, uid string) error {
	return specNotFoundError{
		name:      name,
		namespace: namespace,
		version:   version,
		uid:       uid,
	}
}
