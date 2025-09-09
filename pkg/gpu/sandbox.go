//go:build linux
// +build linux

package gpu

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

// SandboxRunner handles process isolation with mount namespaces and resource limits
type SandboxRunner struct {
	logger *log.Logger
}

// NewSandboxRunner creates a new sandbox runner
func NewSandboxRunner(logger *log.Logger) *SandboxRunner {
	return &SandboxRunner{
		logger: logger,
	}
}

// RunInSandbox executes a command in an isolated sandbox with GPU quotas
func (s *SandboxRunner) RunInSandbox(ctx context.Context, config *GPUShimConfig, cmd []string) error {
	// Check if FUSE is disabled
	if os.Getenv("HYPERCORE_DISABLE_FUSE") != "" {
		s.logger.Printf("FUSE disabled via HYPERCORE_DISABLE_FUSE, skipping mount namespace setup")
		return s.runWithoutSandbox(ctx, config, cmd)
	}

	// Create a new mount namespace
	if err := unix.Unshare(unix.CLONE_NEWNS); err != nil {
		s.logger.Printf("Failed to create mount namespace (continuing without sandbox): %v", err)
		return s.runWithoutSandbox(ctx, config, cmd)
	}

	// Make mounts private so they don't leak
	if err := unix.Mount("", "/", "", unix.MS_REC|unix.MS_PRIVATE, ""); err != nil {
		s.logger.Printf("Failed to make mounts private (continuing without sandbox): %v", err)
		return s.runWithoutSandbox(ctx, config, cmd)
	}

	// Start FUSE meminfo in a temp dir
	tmpBase := filepath.Join(os.TempDir(), fmt.Sprintf("meminfo.%d", os.Getpid()))
	if err := os.MkdirAll(tmpBase, 0755); err != nil {
		return fmt.Errorf("failed to create meminfo temp dir: %w", err)
	}

	// Start FUSE meminfo helper
	fuseCmd := exec.CommandContext(ctx, "python3", "/opt/hypercore/meminfo_fuse.py", tmpBase, fmt.Sprintf("%d", config.CPUMemLimitMB))
	fuseCmd.Stdout = os.Stdout
	fuseCmd.Stderr = os.Stderr
	if err := fuseCmd.Start(); err != nil {
		return fmt.Errorf("failed to start meminfo FUSE: %w", err)
	}

	// Wait for the meminfo file to appear
	meminfoFile := filepath.Join(tmpBase, "meminfo")
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			fuseCmd.Process.Kill()
			return fmt.Errorf("timeout waiting for meminfo file")
		default:
			if _, err := os.Stat(meminfoFile); err == nil {
				goto HAVE_MEMINFO
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
HAVE_MEMINFO:

	// Bind-mount the FUSE meminfo over /proc/meminfo (namespace-local)
	if err := unix.Mount(meminfoFile, "/proc/meminfo", "", unix.MS_BIND, ""); err != nil {
		fuseCmd.Process.Kill()
		return fmt.Errorf("failed to bind mount /proc/meminfo: %w", err)
	}

	// Prepare environment with GPU quotas
	env := os.Environ()
	env = append(env,
		fmt.Sprintf("LD_PRELOAD=/opt/hypercore/libhypercuda.so"),
		fmt.Sprintf("HYPERCORE_VRAM_LIMIT_BYTES=%d", config.VRAMLimitBytes),
		fmt.Sprintf("HYPERCORE_CPU_MEM_MB=%d", config.CPUMemLimitMB),
		fmt.Sprintf("CUDA_VISIBLE_DEVICES=%s", config.CUDAVisibleDevices),
		"PYTHONUNBUFFERED=1",
	)

	// Create the command
	execCmd := exec.CommandContext(ctx, cmd[0], cmd[1:]...)
	execCmd.Env = env
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr
	execCmd.Stdin = os.Stdin

	s.logger.Printf("Starting sandboxed process with VRAM limit: %d bytes, CPU limit: %d MB", 
		config.VRAMLimitBytes, config.CPUMemLimitMB)

	// Start the process
	if err := execCmd.Start(); err != nil {
		fuseCmd.Process.Kill()
		return fmt.Errorf("failed to start sandboxed process: %w", err)
	}

	// Wait for the process to complete
	err := execCmd.Wait()

	// Cleanup
	unix.Unmount("/proc/meminfo", 0)
	fuseCmd.Process.Kill()
	os.RemoveAll(tmpBase)

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return fmt.Errorf("process exited with code %d: %w", exitErr.ExitCode(), err)
		}
		return fmt.Errorf("process failed: %w", err)
	}

	return nil
}

// RunInSandboxWithCgroups runs a command in sandbox with cgroup limits
func (s *SandboxRunner) RunInSandboxWithCgroups(ctx context.Context, config *GPUShimConfig, cmd []string, cgroupPath string) error {
	// This would integrate with cgroups v2 for CPU/memory limits
	// For now, we'll use the basic sandbox
	return s.RunInSandbox(ctx, config, cmd)
}

// runWithoutSandbox runs the command without sandbox isolation
func (s *SandboxRunner) runWithoutSandbox(ctx context.Context, config *GPUShimConfig, cmd []string) error {
	// Prepare environment with GPU quotas
	env := os.Environ()
	env = append(env,
		fmt.Sprintf("LD_PRELOAD=/opt/hypercore/libhypercuda.so"),
		fmt.Sprintf("HYPERCORE_VRAM_LIMIT_BYTES=%d", config.VRAMLimitBytes),
		fmt.Sprintf("HYPERCORE_CPU_MEM_MB=%d", config.CPUMemLimitMB),
		fmt.Sprintf("CUDA_VISIBLE_DEVICES=%s", config.CUDAVisibleDevices),
		"PYTHONUNBUFFERED=1",
	)

	// Create the command
	execCmd := exec.CommandContext(ctx, cmd[0], cmd[1:]...)
	execCmd.Env = env
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr
	execCmd.Stdin = os.Stdin

	s.logger.Printf("Running without sandbox: VRAM limit %d bytes, CPU limit %d MB", 
		config.VRAMLimitBytes, config.CPUMemLimitMB)

	// Start the process
	if err := execCmd.Start(); err != nil {
		return fmt.Errorf("failed to start process: %w", err)
	}

	// Wait for the process to complete
	err := execCmd.Wait()

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return fmt.Errorf("process exited with code %d: %w", exitErr.ExitCode(), err)
		}
		return fmt.Errorf("process failed: %w", err)
	}

	return nil
}

// ValidateSandboxRequirements checks if the system supports sandboxing
func (s *SandboxRunner) ValidateSandboxRequirements() error {
	// Check if we have CAP_SYS_ADMIN
	if os.Geteuid() != 0 {
		s.logger.Printf("Not running as root, sandbox features will be limited")
	}

	// Check if required files exist
	requiredFiles := []string{
		"/opt/hypercore/libhypercuda.so",
	}

	for _, file := range requiredFiles {
		if _, err := os.Stat(file); os.IsNotExist(err) {
			return fmt.Errorf("required file not found: %s", file)
		}
	}

	// Check if python3 is available (only if FUSE is enabled)
	if os.Getenv("HYPERCORE_DISABLE_FUSE") == "" {
		if _, err := exec.LookPath("python3"); err != nil {
			s.logger.Printf("python3 not found, FUSE features will be disabled")
		}
	}

	return nil
}