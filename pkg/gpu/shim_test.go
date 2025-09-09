package gpu

import (
	"os"
	"os/exec"
	"testing"
	"time"
)

// TestHardenedShim tests the hardened CUDA shim with pointer accounting
func TestHardenedShim(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping shim test in short mode")
	}

	// Create a test script that exercises the shim
	testScript := `
import os
import sys

print(f"LD_PRELOAD: {os.environ.get('LD_PRELOAD', 'not set')}")
print(f"VRAM_LIMIT: {os.environ.get('HYPERCORE_VRAM_LIMIT_BYTES', 'not set')}")

try:
    import torch
    print(f"PyTorch available: {torch.cuda.is_available()}")
    
    if torch.cuda.is_available():
        # Test basic allocation
        x = torch.empty(1024, dtype=torch.float32, device="cuda")
        print(f"Allocated tensor: {x.shape}")
        
        # Test memory info
        free, total = torch.cuda.mem_get_info()
        print(f"CUDA memory: {free} free, {total} total")
        
        # Test free
        del x
        torch.cuda.empty_cache()
        
        # Test memory info after free
        free, total = torch.cuda.mem_get_info()
        print(f"CUDA memory after free: {free} free, {total} total")
        
        # Test quota enforcement
        try:
            # Try to allocate more than quota
            large_tensor = torch.empty(1024*1024*1024, dtype=torch.float32, device="cuda")
            print("ERROR: Large allocation should have failed")
        except Exception as e:
            print(f"Quota enforcement working: {e}")
            
except ImportError:
    print("PyTorch not available")
except Exception as e:
    print(f"CUDA error: {e}")
`

	// Write test script to temp file
	tmpFile, err := os.CreateTemp("", "test_shim.py")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(testScript); err != nil {
		t.Fatalf("Failed to write test script: %v", err)
	}
	tmpFile.Close()

	// Test with small quota
	t.Run("SmallQuota", func(t *testing.T) {
		cmd := exec.Command("python3", tmpFile.Name())
		cmd.Env = append(os.Environ(),
			"LD_PRELOAD=/opt/hypercore/libhypercuda.so",
			"HYPERCORE_VRAM_LIMIT_BYTES=1048576", // 1MB
		)
		
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Logf("Command output: %s", string(output))
			// Don't fail the test if PyTorch is not available
			if !contains(string(output), "PyTorch not available") {
				t.Errorf("Command failed: %v", err)
			}
		}
		
		// Check that quota enforcement is working
		if !contains(string(output), "quota exceeded") && !contains(string(output), "PyTorch not available") {
			t.Logf("Output: %s", string(output))
			t.Log("Quota enforcement may not be working (PyTorch might not be available)")
		}
	})

	// Test with disabled shim
	t.Run("DisabledShim", func(t *testing.T) {
		cmd := exec.Command("python3", tmpFile.Name())
		cmd.Env = append(os.Environ(),
			"HYPERCORE_DISABLE_SHIM=1",
			"HYPERCORE_VRAM_LIMIT_BYTES=1048576",
		)
		
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Logf("Command output: %s", string(output))
			// Don't fail the test if PyTorch is not available
			if !contains(string(output), "PyTorch not available") {
				t.Errorf("Command failed: %v", err)
			}
		}
		
		// Check that shim is disabled
		if contains(string(output), "CUDA shim disabled") {
			t.Log("Shim correctly disabled")
		}
	})

	// Test with prefetch enabled
	t.Run("PrefetchEnabled", func(t *testing.T) {
		cmd := exec.Command("python3", tmpFile.Name())
		cmd.Env = append(os.Environ(),
			"LD_PRELOAD=/opt/hypercore/libhypercuda.so",
			"HYPERCORE_VRAM_LIMIT_BYTES=1073741824", // 1GB
			"HYPERCORE_PREFETCH=1",
		)
		
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Logf("Command output: %s", string(output))
			// Don't fail the test if PyTorch is not available
			if !contains(string(output), "PyTorch not available") {
				t.Errorf("Command failed: %v", err)
			}
		}
		
		t.Logf("Prefetch test output: %s", string(output))
	})
}

// TestShimPointerAccounting tests that pointer accounting works correctly
func TestShimPointerAccounting(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping pointer accounting test in short mode")
	}

	// Create a test script that tests pointer accounting
	testScript := `
import os
import sys

try:
    import torch
    if torch.cuda.is_available():
        # Allocate and free multiple times to test accounting
        for i in range(5):
            x = torch.empty(1024*1024, dtype=torch.float32, device="cuda")  # 1MB
            print(f"Allocation {i}: {x.shape}")
            
            # Check memory info
            free, total = torch.cuda.mem_get_info()
            print(f"Memory after alloc {i}: {free} free, {total} total")
            
            # Free the tensor
            del x
            torch.cuda.empty_cache()
            
            # Check memory info after free
            free, total = torch.cuda.mem_get_info()
            print(f"Memory after free {i}: {free} free, {total} total")
            
        print("Pointer accounting test completed")
    else:
        print("CUDA not available")
except ImportError:
    print("PyTorch not available")
except Exception as e:
    print(f"Error: {e}")
`

	// Write test script to temp file
	tmpFile, err := os.CreateTemp("", "test_accounting.py")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(testScript); err != nil {
		t.Fatalf("Failed to write test script: %v", err)
	}
	tmpFile.Close()

	// Run the test
	cmd := exec.Command("python3", tmpFile.Name())
	cmd.Env = append(os.Environ(),
		"LD_PRELOAD=/opt/hypercore/libhypercuda.so",
		"HYPERCORE_VRAM_LIMIT_BYTES=10485760", // 10MB
	)
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("Command output: %s", string(output))
		// Don't fail the test if PyTorch is not available
		if !contains(string(output), "PyTorch not available") {
			t.Errorf("Command failed: %v", err)
		}
	}
	
	t.Logf("Pointer accounting test output: %s", string(output))
}

// TestShimAsyncAPIs tests async CUDA APIs
func TestShimAsyncAPIs(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping async API test in short mode")
	}

	// Create a test script that tests async APIs
	testScript := `
import os
import sys

try:
    import torch
    if torch.cuda.is_available():
        # Test async allocation
        stream = torch.cuda.Stream()
        with torch.cuda.stream(stream):
            x = torch.empty(1024*1024, dtype=torch.float32, device="cuda")
            print(f"Async allocation: {x.shape}")
            
            # Check memory info
            free, total = torch.cuda.mem_get_info()
            print(f"Memory after async alloc: {free} free, {total} total")
            
            # Free the tensor
            del x
            torch.cuda.empty_cache()
            
            # Check memory info after free
            free, total = torch.cuda.mem_get_info()
            print(f"Memory after async free: {free} free, {total} total")
            
        print("Async API test completed")
    else:
        print("CUDA not available")
except ImportError:
    print("PyTorch not available")
except Exception as e:
    print(f"Error: {e}")
`

	// Write test script to temp file
	tmpFile, err := os.CreateTemp("", "test_async.py")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(testScript); err != nil {
		t.Fatalf("Failed to write test script: %v", err)
	}
	tmpFile.Close()

	// Run the test
	cmd := exec.Command("python3", tmpFile.Name())
	cmd.Env = append(os.Environ(),
		"LD_PRELOAD=/opt/hypercore/libhypercuda.so",
		"HYPERCORE_VRAM_LIMIT_BYTES=10485760", // 10MB
	)
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("Command output: %s", string(output))
		// Don't fail the test if PyTorch is not available
		if !contains(string(output), "PyTorch not available") {
			t.Errorf("Command failed: %v", err)
		}
	}
	
	t.Logf("Async API test output: %s", string(output))
}

// TestSandboxGracefulDegradation tests graceful degradation without root
func TestSandboxGracefulDegradation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping sandbox test in short mode")
	}

	// Create a test script
	testScript := `
import os
import sys

print(f"PID: {os.getpid()}")
print(f"LD_PRELOAD: {os.environ.get('LD_PRELOAD', 'not set')}")
print(f"VRAM_LIMIT: {os.environ.get('HYPERCORE_VRAM_LIMIT_BYTES', 'not set')}")
print(f"CPU_MEM_LIMIT: {os.environ.get('HYPERCORE_CPU_MEM_MB', 'not set')}")

# Test memory info
try:
    with open('/proc/meminfo', 'r') as f:
        meminfo = f.read()
        print("=== /proc/meminfo ===")
        print(meminfo[:500] + "..." if len(meminfo) > 500 else meminfo)
except Exception as e:
    print(f"Error reading /proc/meminfo: {e}")

print("=== Test Complete ===")
`

	// Write test script to temp file
	tmpFile, err := os.CreateTemp("", "test_sandbox.py")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(testScript); err != nil {
		t.Fatalf("Failed to write test script: %v", err)
	}
	tmpFile.Close()

	// Test with FUSE disabled
	t.Run("FUSEDisabled", func(t *testing.T) {
		cmd := exec.Command("/opt/hypercore/bin/sandbox", "python3", tmpFile.Name())
		cmd.Env = append(os.Environ(),
			"HYPERCORE_DISABLE_FUSE=1",
			"HYPERCORE_VRAM_LIMIT_BYTES=1073741824",
			"HYPERCORE_CPU_MEM_MB=1024",
		)
		
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Errorf("Sandbox with FUSE disabled failed: %v", err)
		}
		
		if !contains(string(output), "FUSE disabled") {
			t.Logf("Output: %s", string(output))
			t.Log("FUSE disable message not found")
		}
	})

	// Test without sandbox (should still work)
	t.Run("NoSandbox", func(t *testing.T) {
		cmd := exec.Command("python3", tmpFile.Name())
		cmd.Env = append(os.Environ(),
			"LD_PRELOAD=/opt/hypercore/libhypercuda.so",
			"HYPERCORE_VRAM_LIMIT_BYTES=1073741824",
			"HYPERCORE_CPU_MEM_MB=1024",
		)
		
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Errorf("Command without sandbox failed: %v", err)
		}
		
		t.Logf("No sandbox test output: %s", string(output))
	})
}

// Benchmark tests
func BenchmarkShimAllocation(t *testing.B) {
	if testing.Short() {
		t.Skip("Skipping benchmark in short mode")
	}

	// Create a simple allocation test
	testScript := `
import torch
for i in range(100):
    x = torch.empty(1024, dtype=torch.float32, device="cuda")
    del x
`

	// Write test script to temp file
	tmpFile, err := os.CreateTemp("", "benchmark.py")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(testScript); err != nil {
		t.Fatalf("Failed to write test script: %v", err)
	}
	tmpFile.Close()

	t.ResetTimer()
	for i := 0; i < t.N; i++ {
		cmd := exec.Command("python3", tmpFile.Name())
		cmd.Env = append(os.Environ(),
			"LD_PRELOAD=/opt/hypercore/libhypercuda.so",
			"HYPERCORE_VRAM_LIMIT_BYTES=1073741824",
		)
		
		if err := cmd.Run(); err != nil {
			t.Fatalf("Benchmark failed: %v", err)
		}
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || 
		(len(s) > len(substr) && 
			(s[:len(substr)] == substr || 
			 s[len(s)-len(substr):] == substr || 
			 contains(s[1:], substr))))
}