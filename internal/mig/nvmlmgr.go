//go:build linux
// +build linux

package mig

/*
#cgo LDFLAGS: -lnvidia-ml
#include <stdlib.h>
#include <nvml.h>

static const char* nvmlErrorStringWrap(nvmlReturn_t r) {
    return nvmlErrorString(r);
}
*/
import "C"

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
	"unsafe"
)

// Device represents a GPU device (physical or MIG)
type Device struct {
	GPUIndex int    // Parent GPU index
	UUID     string // MIG-GPU-... if MIG device; GPU-... if physical
	MemoryB  uint64 // Total memory in bytes
	IsMIG    bool   // True if this is a MIG device
}

// nvErr converts NVML return code to Go error
func nvErr(r C.nvmlReturn_t, where string) error {
	if r == C.NVML_SUCCESS {
		return nil
	}
	return fmt.Errorf("%s: %s", where, C.GoString(C.nvmlErrorStringWrap(r)))
}

// Init initializes the NVML library
func Init() error {
	return nvErr(C.nvmlInit_v2(), "nvmlInit")
}

// Shutdown shuts down the NVML library
func Shutdown() {
	_ = C.nvmlShutdown()
}

// getUUID retrieves the UUID of a device
func getUUID(dev C.nvmlDevice_t) (string, error) {
	var cstr *C.char
	r := C.nvmlDeviceGetUUID(dev, &cstr, 96)
	if err := nvErr(r, "nvmlDeviceGetUUID"); err != nil {
		return "", err
	}
	return C.GoString(cstr), nil
}

// getMemBytes retrieves the total memory of a device
func getMemBytes(dev C.nvmlDevice_t) (uint64, error) {
	var mem C.nvmlMemory_t
	r := C.nvmlDeviceGetMemoryInfo(dev, &mem)
	if err := nvErr(r, "nvmlDeviceGetMemoryInfo"); err != nil {
		return 0, err
	}
	return uint64(mem.total), nil
}

// ListAll enumerates all physical GPUs and their MIG devices
func ListAll() ([]Device, error) {
	if err := Init(); err != nil {
		return nil, err
	}
	defer Shutdown()

	var count C.uint
	if err := nvErr(C.nvmlDeviceGetCount_v2(&count), "nvmlDeviceGetCount"); err != nil {
		return nil, err
	}

	var out []Device
	for i := 0; i < int(count); i++ {
		var h C.nvmlDevice_t
		if err := nvErr(C.nvmlDeviceGetHandleByIndex_v2(C.uint(i), &h), "nvmlDeviceGetHandleByIndex"); err != nil {
			return nil, err
		}

		// Physical GPU
		uuid, err := getUUID(h)
		if err != nil {
			return nil, err
		}
		mem, err := getMemBytes(h)
		if err != nil {
			return nil, err
		}
		out = append(out, Device{GPUIndex: i, UUID: uuid, MemoryB: mem, IsMIG: false})

		// Check if MIG mode is enabled
		var cur, supported C.nvmlEnableState_t
		if err := nvErr(C.nvmlDeviceGetMigMode(h, &cur, &supported), "nvmlDeviceGetMigMode"); err == nil && cur == C.NVML_FEATURE_ENABLED {
			// Enumerate MIG devices
			var max C.uint
			if err := nvErr(C.nvmlDeviceGetMaxMigDeviceCount(h, &max), "nvmlDeviceGetMaxMigDeviceCount"); err != nil {
				return nil, err
			}
			for m := C.uint(0); m < max; m++ {
				var mh C.nvmlDevice_t
				r := C.nvmlDeviceGetMigDeviceHandleByIndex(h, m, &mh)
				if r == C.NVML_SUCCESS {
					mUUID, err := getUUID(mh) // e.g., MIG-GPU-...
					if err != nil {
						return nil, err
					}
					mMem, err := getMemBytes(mh) // e.g., ~10GiB for 1g.10gb
					if err != nil {
						return nil, err
					}
					out = append(out, Device{GPUIndex: i, UUID: mUUID, MemoryB: mMem, IsMIG: true})
				} else if r == C.NVML_ERROR_NOT_FOUND {
					// No more MIG devices in use at this index
					continue
				}
			}
		}
	}
	return out, nil
}

// Reservation constants
const lockDir = "/var/run/hypercore/mig"

// ReserveByProfileMB reserves a MIG device that meets the memory requirement
func ReserveByProfileMB(profileMB uint64, ttl time.Duration) (dev Device, release func(), err error) {
	devs, err := ListAll()
	if err != nil {
		return Device{}, nil, err
	}

	// Find closest >= profileMB among MIG devices
	var best *Device
	for i := range devs {
		d := &devs[i]
		if !d.IsMIG {
			continue
		}
		mb := d.MemoryB / (1024 * 1024)
		if mb >= profileMB {
			if best == nil || mb < (best.MemoryB/(1024*1024)) {
				best = d
			}
		}
	}
	if best == nil {
		return Device{}, nil, errors.New("no MIG device meeting profileMB requirement")
	}

	// Create lock directory
	if err := os.MkdirAll(lockDir, 0755); err != nil {
		return Device{}, nil, err
	}
	lp := filepath.Join(lockDir, best.UUID+".lock")

	// Garbage collect stale locks
	if st, err := os.Stat(lp); err == nil {
		if time.Since(st.ModTime()) > ttl {
			_ = os.Remove(lp)
		}
	}

	// Create lock file
	f, err := os.OpenFile(lp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return Device{}, nil, fmt.Errorf("reserve: %w", err)
	}
	defer f.Close()
	_, _ = f.Write([]byte(fmt.Sprintf("pid=%d ts=%s\n", os.Getpid(), time.Now().Format(time.RFC3339))))

	return *best, func() { _ = os.Remove(lp) }, nil
}

// GuessProfile attempts to guess the MIG profile from memory size
func GuessProfile(memB uint64) string {
	mb := memB / (1024 * 1024)
	switch {
	case mb >= 19500 && mb <= 20500:
		return "2g.20gb?"
	case mb >= 9800 && mb <= 10240:
		return "1g.10gb?"
	case mb >= 4900 && mb <= 5150:
		return "1g.5gb?"
	case mb >= 49000 && mb <= 51500:
		return "4g.40gb?"
	case mb >= 24500 && mb <= 25750:
		return "2g.20gb?"
	default:
		return fmt.Sprintf("~%d MB", mb)
	}
}

// cString creates a C string from Go string (utility function)
func cString(s string) *C.char {
	return C.CString(s)
}

// freeCString frees a C string (utility function)
func freeCString(p *C.char) {
	C.free(unsafe.Pointer(p))
}