# MIG Testing Guide

This guide explains how to set up and run end-to-end testing for the MIG (Multi-Instance GPU) server implementation.

## Prerequisites

Before running the tests, ensure you have:

1. A GCP VM with NVIDIA GPU support
2. The MIG server installed and running
3. `jq` installed (the script will install it if missing)
4. Proper permissions to execute shell scripts

## Setting Up Test Environment

1. Clone the repository (if not already done):
   ```bash
   git clone https://github.com/vistara/hypercore.git
   cd hypercore
   ```

2. Switch to the testing branch:
   ```bash
   git fetch
   git checkout feature/mig-testing-guide
   ```

3. Make scripts executable:
   ```bash
   chmod +x deploy-mig-gcp.sh test-mig-end-to-end.sh
   ```

## Deployment

1. First, deploy the MIG server if not already done:
   ```bash
   ./deploy-mig-gcp.sh
   ```

2. Wait for deployment to complete and verify the server is running:
   ```bash
   sudo systemctl status mig-server
   ```

## Running End-to-End Tests

1. Execute the test script:
   ```bash
   ./test-mig-end-to-end.sh
   ```

The script will run through the following tests:

- Health Check
- Device Discovery
- GPU Allocation
- Allocation Status
- Device Utilization
- All Allocations
- Device Status
- GPU Deallocation

## Test Results

The script will provide a summary of test results, including:
- Total number of tests run
- Number of tests passed
- Number of tests failed
- Success rate percentage

### Success Scenario
If all tests pass, you'll see:
```
🎉 All tests passed! MIG implementation is working correctly.
```

### Failure Scenario
If any tests fail:
1. Check the test output for specific failures
2. Review the MIG server logs:
   ```bash
   sudo journalctl -u mig-server -f
   ```

## Test Components

### 1. Health Check
Verifies the MIG server is running and responding to basic health checks.

### 2. Device Discovery
Tests the ability to discover and list available GPU devices.

### 3. GPU Allocation
Tests allocation of GPU resources with specific profiles:
- Memory: 5GB
- Compute Utilization: 1 GPU

### 4. Allocation Status
Verifies the status of allocated GPU resources and workload tracking.

### 5. Device Utilization
Monitors and reports GPU device utilization metrics.

### 6. All Allocations
Lists and verifies all current GPU allocations.

### 7. Device Status
Provides comprehensive status including devices, utilization, and allocations.

### 8. GPU Deallocation
Tests proper cleanup and release of GPU resources.

## Troubleshooting

1. If the MIG server isn't running:
   ```bash
   sudo systemctl start mig-server
   ```

2. If scripts aren't executable:
   ```bash
   chmod +x *.sh
   ```

3. If `jq` is missing:
   ```bash
   sudo apt-get update
   sudo apt-get install -y jq
   ```

4. For permission issues:
   ```bash
   # Check script ownership
   ls -l *.sh
   # Fix permissions if needed
   sudo chown $USER:$USER *.sh
   ```

## Additional Resources

- Check MIG server logs:
  ```bash
  sudo journalctl -u mig-server -f
  ```
- Monitor GPU status:
  ```bash
  nvidia-smi
  ```
- View real-time GPU metrics:
  ```bash
  nvidia-smi dmon
  ```
