#!/bin/bash

# End-to-End MIG Testing Script
# Tests the complete MIG functionality

set -euo pipefail

echo "🧪 MIG End-to-End Testing"
echo "========================"

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

print_colored() {
    local color=$1
    local message=$2
    echo -e "${color}${message}${NC}"
}

# Configuration
API_BASE="http://localhost:8080/api/v1"
TEST_WORKLOAD_ID="test-workload-$(date +%s)"

# Test functions
test_health_check() {
    print_colored $BLUE "🏥 Testing health check..."
    if curl -f -s "$API_BASE/health" > /dev/null; then
        print_colored $GREEN "✅ Health check passed"
        return 0
    else
        print_colored $RED "❌ Health check failed"
        return 1
    fi
}

test_device_discovery() {
    print_colored $BLUE "🔍 Testing device discovery..."
    response=$(curl -s "$API_BASE/gpu/devices")
    if echo "$response" | jq -e '. | length >= 0' > /dev/null 2>&1; then
        device_count=$(echo "$response" | jq '. | length')
        print_colored $GREEN "✅ Device discovery passed - Found $device_count devices"
        echo "$response" | jq '.'
        return 0
    else
        print_colored $RED "❌ Device discovery failed"
        return 1
    fi
}

test_gpu_allocation() {
    print_colored $BLUE "🎯 Testing GPU allocation..."
    
    # Test allocation request
    allocation_request='{
        "workload_id": "'$TEST_WORKLOAD_ID'",
        "profile": {
            "id": "1g.5gb",
            "memory_gb": 5,
            "compute_util": 1
        }
    }'
    
    response=$(curl -s -X POST "$API_BASE/gpu/allocate" \
        -H "Content-Type: application/json" \
        -d "$allocation_request")
    
    if echo "$response" | jq -e '.success == true' > /dev/null 2>&1; then
        print_colored $GREEN "✅ GPU allocation successful"
        echo "$response" | jq '.'
        return 0
    else
        print_colored $RED "❌ GPU allocation failed"
        echo "$response" | jq '.'
        return 1
    fi
}

test_allocation_status() {
    print_colored $BLUE "📊 Testing allocation status..."
    
    response=$(curl -s "$API_BASE/gpu/allocation/$TEST_WORKLOAD_ID")
    
    if echo "$response" | jq -e '.workload_id == "'$TEST_WORKLOAD_ID'"' > /dev/null 2>&1; then
        print_colored $GREEN "✅ Allocation status check passed"
        echo "$response" | jq '.'
        return 0
    else
        print_colored $RED "❌ Allocation status check failed"
        echo "$response" | jq '.'
        return 1
    fi
}

test_device_utilization() {
    print_colored $BLUE "📈 Testing device utilization..."
    
    response=$(curl -s "$API_BASE/gpu/devices/utilization")
    
    if echo "$response" | jq -e '. | type == "object"' > /dev/null 2>&1; then
        print_colored $GREEN "✅ Device utilization check passed"
        echo "$response" | jq '.'
        return 0
    else
        print_colored $RED "❌ Device utilization check failed"
        echo "$response" | jq '.'
        return 1
    fi
}

test_gpu_deallocation() {
    print_colored $BLUE "🗑️  Testing GPU deallocation..."
    
    response=$(curl -s -X DELETE "$API_BASE/gpu/deallocate/$TEST_WORKLOAD_ID")
    
    if [ "$response" = "GPU deallocated successfully for workload $TEST_WORKLOAD_ID" ]; then
        print_colored $GREEN "✅ GPU deallocation successful"
        return 0
    else
        print_colored $RED "❌ GPU deallocation failed"
        echo "Response: $response"
        return 1
    fi
}

test_all_allocations() {
    print_colored $BLUE "📋 Testing all allocations list..."
    
    response=$(curl -s "$API_BASE/gpu/allocations")
    
    if echo "$response" | jq -e '. | type == "object"' > /dev/null 2>&1; then
        print_colored $GREEN "✅ All allocations check passed"
        echo "$response" | jq '.'
        return 0
    else
        print_colored $RED "❌ All allocations check failed"
        echo "$response" | jq '.'
        return 1
    fi
}

test_device_status() {
    print_colored $BLUE "🔧 Testing device status..."
    
    response=$(curl -s "$API_BASE/gpu/devices/status")
    
    if echo "$response" | jq -e '.devices and .utilization and .allocations' > /dev/null 2>&1; then
        print_colored $GREEN "✅ Device status check passed"
        echo "$response" | jq '.'
        return 0
    else
        print_colored $RED "❌ Device status check failed"
        echo "$response" | jq '.'
        return 1
    fi
}

# Check if jq is installed
if ! command -v jq &> /dev/null; then
    print_colored $YELLOW "⚠️  Installing jq for JSON parsing..."
    sudo apt-get update
    sudo apt-get install -y jq
fi

# Check if MIG server is running
if ! curl -f -s "$API_BASE/health" > /dev/null; then
    print_colored $RED "❌ MIG server is not running. Please start it first:"
    echo "  sudo systemctl start mig-server"
    exit 1
fi

# Run tests
print_colored $GREEN "🚀 Starting MIG End-to-End Tests"
echo "=================================="

test_count=0
passed_count=0

# Test 1: Health Check
((test_count++))
if test_health_check; then
    ((passed_count++))
fi
echo ""

# Test 2: Device Discovery
((test_count++))
if test_device_discovery; then
    ((passed_count++))
fi
echo ""

# Test 3: GPU Allocation
((test_count++))
if test_gpu_allocation; then
    ((passed_count++))
fi
echo ""

# Test 4: Allocation Status
((test_count++))
if test_allocation_status; then
    ((passed_count++))
fi
echo ""

# Test 5: Device Utilization
((test_count++))
if test_device_utilization; then
    ((passed_count++))
fi
echo ""

# Test 6: All Allocations
((test_count++))
if test_all_allocations; then
    ((passed_count++))
fi
echo ""

# Test 7: Device Status
((test_count++))
if test_device_status; then
    ((passed_count++))
fi
echo ""

# Test 8: GPU Deallocation
((test_count++))
if test_gpu_deallocation; then
    ((passed_count++))
fi
echo ""

# Test Summary
print_colored $GREEN "📊 Test Summary"
echo "=================================="
echo "Total Tests: $test_count"
echo "Passed: $passed_count"
echo "Failed: $((test_count - passed_count))"
echo "Success Rate: $((passed_count * 100 / test_count))%"

if [ $passed_count -eq $test_count ]; then
    print_colored $GREEN "🎉 All tests passed! MIG implementation is working correctly."
    exit 0
else
    print_colored $RED "❌ Some tests failed. Please check the MIG server logs:"
    echo "  sudo journalctl -u mig-server -f"
    exit 1
fi