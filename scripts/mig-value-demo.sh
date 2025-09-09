#!/bin/bash

# MIG Value Demonstration Script
# Shows the key capabilities and value propositions of MIG implementation

set -euo pipefail

# Color codes for better visualization
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

# 1. Dynamic GPU Partitioning
demo_gpu_partitioning() {
    print_colored $BLUE "🎯 1. Dynamic GPU Partitioning"
    echo "============================================"
    echo "Demonstrating how a single A100 GPU can be split into multiple instances:"
    
    # Show available GPU
    print_colored $YELLOW "👉 Checking available GPU resources..."
    curl -s "$API_BASE/gpu/devices" | jq '.'
    
    # Create different partition sizes
    print_colored $YELLOW "👉 Creating multiple GPU partitions..."
    
    # Allocate 3GB partition
    curl -s -X POST "$API_BASE/gpu/allocate" -H "Content-Type: application/json" \
        -d '{
            "workload_id": "small-llm",
            "profile": {
                "id": "1g.3gb",
                "memory_gb": 3,
                "compute_util": 1
            }
        }' | jq '.'
        
    # Allocate 5GB partition
    curl -s -X POST "$API_BASE/gpu/allocate" -H "Content-Type: application/json" \
        -d '{
            "workload_id": "medium-llm",
            "profile": {
                "id": "1g.5gb",
                "memory_gb": 5,
                "compute_util": 1
            }
        }' | jq '.'
        
    # Show current allocations
    print_colored $YELLOW "👉 Current GPU partitions:"
    curl -s "$API_BASE/gpu/allocations" | jq '.'
}

# 2. Multi-Tenant Efficiency
demo_multi_tenancy() {
    print_colored $BLUE "🏢 2. Multi-Tenant Efficiency"
    echo "============================================"
    echo "Running multiple AI models simultaneously on the same GPU:"
    
    # Show resource utilization
    print_colored $YELLOW "👉 Current GPU utilization:"
    curl -s "$API_BASE/gpu/devices/utilization" | jq '.'
    
    # Show isolation metrics
    print_colored $YELLOW "👉 Memory isolation between tenants:"
    curl -s "$API_BASE/gpu/devices/status" | jq '.allocations'
}

# 3. Cost Optimization
demo_cost_benefits() {
    print_colored $BLUE "💰 3. Cost Optimization"
    echo "============================================"
    
    # Calculate theoretical cost savings
    A100_COST_PER_HOUR=2.99
    
    echo "Cost Analysis for A100 GPU:"
    echo "- Full A100 GPU cost per hour: \$${A100_COST_PER_HOUR}"
    echo "- With MIG partitioning (4 instances):"
    echo "  * Cost per partition: \$$(echo "scale=2; ${A100_COST_PER_HOUR}/4" | bc) per hour"
    echo "  * Potential savings: Up to 75% for smaller workloads"
    
    # Show current efficiency
    print_colored $YELLOW "👉 Current resource efficiency:"
    curl -s "$API_BASE/gpu/devices/utilization" | jq '.'
}

# 4. Resource Optimization
demo_resource_optimization() {
    print_colored $BLUE "⚡ 4. Resource Optimization"
    echo "============================================"
    
    # Show automatic scaling
    print_colored $YELLOW "👉 Demonstrating dynamic resource allocation:"
    
    # Release unused resources
    print_colored $YELLOW "Deallocating unused partitions..."
    curl -s -X DELETE "$API_BASE/gpu/deallocate/small-llm"
    
    # Show updated allocation
    print_colored $YELLOW "👉 Updated resource allocation:"
    curl -s "$API_BASE/gpu/allocations" | jq '.'
}

# Main demo flow
main() {
    print_colored $GREEN "🚀 MIG Implementation Value Proposition Demo"
    echo "=================================================="
    echo "This demo showcases how MIG implementation transforms GPU utilization:"
    echo ""
    
    # Run each demonstration
    demo_gpu_partitioning
    echo ""
    sleep 2
    
    demo_multi_tenancy
    echo ""
    sleep 2
    
    demo_cost_benefits
    echo ""
    sleep 2
    
    demo_resource_optimization
    echo ""
    
    # Summary
    print_colored $GREEN "✨ Key Value Propositions Demonstrated:"
    echo "1. Dynamic GPU Partitioning"
    echo "   - Split single GPU into multiple instances"
    echo "   - Flexible memory allocation"
    echo "   - Independent compute resources"
    echo ""
    echo "2. Multi-Tenant Efficiency"
    echo "   - Run multiple models simultaneously"
    echo "   - Complete memory isolation"
    echo "   - Independent scaling"
    echo ""
    echo "3. Cost Optimization"
    echo "   - Reduced cost per workload"
    echo "   - Better resource utilization"
    echo "   - Pay only for needed resources"
    echo ""
    echo "4. Resource Optimization"
    echo "   - Dynamic resource allocation"
    echo "   - Automatic scaling"
    echo "   - Efficient resource management"
}

# Run the demo
main
