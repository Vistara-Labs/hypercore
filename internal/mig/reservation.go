package mig

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

// Profile represents a MIG profile with memory requirements
type Profile struct {
	Name    string // e.g., "1g.10gb", "2g.20gb"
	MemoryMB uint64 // Required memory in MB
}

// Common MIG profiles
var (
	Profile1g5gb  = Profile{Name: "1g.5gb", MemoryMB: 5120}   // ~5GB
	Profile1g10gb = Profile{Name: "1g.10gb", MemoryMB: 10240} // ~10GB
	Profile2g20gb = Profile{Name: "2g.20gb", MemoryMB: 20480} // ~20GB
	Profile4g40gb = Profile{Name: "4g.40gb", MemoryMB: 40960} // ~40GB
)

// ParseProfile parses a profile string like "1g.10gb" into a Profile
func ParseProfile(profileStr string) (Profile, error) {
	profileStr = strings.ToLower(strings.TrimSpace(profileStr))
	
	switch profileStr {
	case "1g.5gb":
		return Profile1g5gb, nil
	case "1g.10gb":
		return Profile1g10gb, nil
	case "2g.20gb":
		return Profile2g20gb, nil
	case "4g.40gb":
		return Profile4g40gb, nil
	default:
		// Try to parse custom profile like "custom.10240" for 10240MB
		parts := strings.Split(profileStr, ".")
		if len(parts) == 2 && parts[0] == "custom" {
			if mb, err := strconv.ParseUint(parts[1], 10, 64); err == nil {
				return Profile{Name: profileStr, MemoryMB: mb}, nil
			}
		}
		return Profile{}, fmt.Errorf("unknown MIG profile: %s", profileStr)
	}
}

// ReserveByProfile reserves a MIG device by profile name
func ReserveByProfile(profileStr string, ttl time.Duration) (Device, func(), error) {
	profile, err := ParseProfile(profileStr)
	if err != nil {
		return Device{}, nil, err
	}
	
	return ReserveByProfileMB(profile.MemoryMB, ttl)
}

// ReserveFromEnv reserves a MIG device based on environment variables
func ReserveFromEnv(logger *log.Logger) (Device, func(), error) {
	// Check for MIG profile environment variable
	profileStr := os.Getenv("HC_MIG_PROFILE")
	if profileStr == "" {
		return Device{}, nil, fmt.Errorf("HC_MIG_PROFILE not set")
	}
	
	// Parse TTL from environment (default 5 minutes)
	ttlStr := os.Getenv("HC_MIG_TTL")
	ttl := 5 * time.Minute
	if ttlStr != "" {
		if parsed, err := time.ParseDuration(ttlStr); err == nil {
			ttl = parsed
		}
	}
	
	logger.Printf("Attempting to reserve MIG profile: %s (TTL: %v)", profileStr, ttl)
	
	dev, release, err := ReserveByProfile(profileStr, ttl)
	if err != nil {
		logger.Printf("MIG reservation failed: %v", err)
		return Device{}, nil, err
	}
	
	logger.Printf("Successfully reserved MIG device: UUID=%s, GPU=%d, Memory=%dB, Profile=%s", 
		dev.UUID, dev.GPUIndex, dev.MemoryB, GuessProfile(dev.MemoryB))
	
	return dev, release, nil
}

// ListAvailableProfiles returns available MIG devices grouped by profile
func ListAvailableProfiles() (map[string][]Device, error) {
	devs, err := ListAll()
	if err != nil {
		return nil, err
	}
	
	profiles := make(map[string][]Device)
	for _, dev := range devs {
		if dev.IsMIG {
			profile := GuessProfile(dev.MemoryB)
			profiles[profile] = append(profiles[profile], dev)
		}
	}
	
	return profiles, nil
}