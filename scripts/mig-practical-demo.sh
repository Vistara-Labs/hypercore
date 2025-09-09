#!/bin/bash

# MIG Practical Use Cases Demo
# Demonstrates real-world applications of MIG technology

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

# Helper function to wait for effect
demo_pause() {
    sleep 2
}

# Check prerequisites
check_prerequisites() {
    print_colored $BLUE "🔍 Checking Prerequisites"
    echo "=============================="
    
    # Check if NVIDIA drivers are working
    if ! command -v nvidia-smi &> /dev/null; then
        print_colored $RED "❌ NVIDIA drivers not found!"
        exit 1
    fi
    
    # Check if MIG server is running
    if ! curl -s "$API_BASE/health" &> /dev/null; then
        print_colored $RED "❌ MIG server not running!"
        exit 1
    fi
    
    print_colored $GREEN "✅ System ready for demo"
    echo ""
}

# Use Case 1: AI Model Development Team
demo_model_development() {
    print_colored $BLUE "👥 Use Case 1: AI Model Development Team"
    echo "========================================"
    echo "Scenario: Multiple data scientists working on different models"
    
    # Allocate resources for multiple developers
    print_colored $YELLOW "Creating development environments..."
    
    # Developer 1: Training a small NLP model
    curl -s -X POST "$API_BASE/gpu/allocate" -H "Content-Type: application/json" \
        -d '{
            "workload_id": "dev1-nlp-training",
            "profile": {
                "id": "1g.5gb",
                "memory_gb": 5,
                "compute_util": 1
            }
        }' | jq '.'
    
    # Developer 2: Fine-tuning a vision model
    curl -s -X POST "$API_BASE/gpu/allocate" -H "Content-Type: application/json" \
        -d '{
            "workload_id": "dev2-vision-tuning",
            "profile": {
                "id": "1g.10gb",
                "memory_gb": 10,
                "compute_util": 1
            }
        }' | jq '.'
    
    # Show resource allocation
    print_colored $YELLOW "Current development environment status:"
    curl -s "$API_BASE/gpu/allocations" | jq '.'
    
    demo_pause
}

# Use Case 2: Production Model Serving
demo_model_serving() {
    print_colored $BLUE "🚀 Use Case 2: Production Model Serving"
    echo "========================================"
    echo "Scenario: Serving multiple production models efficiently"
    
    # Allocate resources for production models
    print_colored $YELLOW "Deploying production models..."
    
    # LLM API Service
    curl -s -X POST "$API_BASE/gpu/allocate" -H "Content-Type: application/json" \
        -d '{
            "workload_id": "prod-llm-api",
            "profile": {
                "id": "1g.20gb",
                "memory_gb": 20,
                "compute_util": 1
            }
        }' | jq '.'
    
    # Image Processing Service
    curl -s -X POST "$API_BASE/gpu/allocate" -H "Content-Type: application/json" \
        -d '{
            "workload_id": "prod-image-api",
            "profile": {
                "id": "1g.8gb",
                "memory_gb": 8,
                "compute_util": 1
            }
        }' | jq '.'
    
    # Show production workload status
    print_colored $YELLOW "Production services status:"
    curl -s "$API_BASE/gpu/devices/status" | jq '.'
    
    demo_pause
}

# Use Case 3: Resource Optimization
demo_resource_optimization() {
    print_colored $BLUE "⚡ Use Case 3: Resource Optimization"
    echo "========================================"
    echo "Scenario: Dynamic resource reallocation based on demand"
    
    # Show current utilization
    print_colored $YELLOW "Current GPU utilization:"
    curl -s "$API_BASE/gpu/devices/utilization" | jq '.'
    
    # Simulate peak hours
    print_colored $YELLOW "Simulating peak load handling..."
    
    # Scale up LLM service
    curl -s -X POST "$API_BASE/gpu/allocate" -H "Content-Type: application/json" \
        -d '{
            "workload_id": "prod-llm-api-replica",
            "profile": {
                "id": "1g.20gb",
                "memory_gb": 20,
                "compute_util": 1
            }
        }' | jq '.'
    
    # Show updated allocation
    print_colored $YELLOW "Updated resource allocation:"
    curl -s "$API_BASE/gpu/allocations" | jq '.'
    
    demo_pause
}

# Use Case 4: Cost Analysis
demo_cost_analysis() {
    print_colored $BLUE "💰 Use Case 4: Cost Analysis"
    echo "========================================"
    echo "Scenario: Analyzing cost benefits of MIG implementation"
    
    # Constants for calculation
    A100_COST_PER_HOUR=2.99
    RUNNING_WORKLOADS=$(curl -s "$API_BASE/gpu/allocations" | jq '. | length')
    
    # Calculate costs
    print_colored $YELLOW "Cost Analysis for Current Workloads:"
    echo "Traditional Setup (1 GPU per workload):"
    echo "- Number of GPUs needed: $RUNNING_WORKLOADS"
    echo "- Cost per hour: \$$(echo "scale=2; ${A100_COST_PER_HOUR} * ${RUNNING_WORKLOADS}" | bc)"
    echo "- Cost per month: \$$(echo "scale=2; ${A100_COST_PER_HOUR} * ${RUNNING_WORKLOADS} * 24 * 30" | bc)"
    echo ""
    echo "With MIG:"
    echo "- Number of GPUs needed: 1"
    echo "- Cost per hour: \$${A100_COST_PER_HOUR}"
    echo "- Cost per month: \$$(echo "scale=2; ${A100_COST_PER_HOUR} * 24 * 30" | bc)"
    echo "- Monthly savings: \$$(echo "scale=2; (${A100_COST_PER_HOUR} * ${RUNNING_WORKLOADS} * 24 * 30) - (${A100_COST_PER_HOUR} * 24 * 30)" | bc)"
    
    demo_pause
}

# Use Case 5: Cleanup and Resource Management
demo_cleanup() {
    print_colored $BLUE "🧹 Use Case 5: Cleanup and Resource Management"
    echo "========================================"
    echo "Scenario: Efficient resource cleanup and reallocation"
    
    # Show current state
    print_colored $YELLOW "Current allocations before cleanup:"
    curl -s "$API_BASE/gpu/allocations" | jq '.'
    
    # Cleanup development resources
    print_colored $YELLOW "Cleaning up development resources..."
    curl -s -X DELETE "$API_BASE/gpu/deallocate/dev1-nlp-training"
    curl -s -X DELETE "$API_BASE/gpu/deallocate/dev2-vision-tuning"
    
    # Cleanup production resources
    print_colored $YELLOW "Cleaning up production resources..."
    curl -s -X DELETE "$API_BASE/gpu/deallocate/prod-llm-api"
    curl -s -X DELETE "$API_BASE/gpu/deallocate/prod-image-api"
    curl -s -X DELETE "$API_BASE/gpu/deallocate/prod-llm-api-replica"
    
    # Show final state
    print_colored $YELLOW "Final system state:"
    curl -s "$API_BASE/gpu/allocations" | jq '.'
    
    demo_pause
}

# Main demo flow
main() {
    clear
    print_colored $GREEN "🎯 MIG Real-World Use Cases Demo"
    echo "=================================="
    echo ""
    
    # Check system readiness
    check_prerequisites
    
    # Run demonstrations
    demo_model_development
    echo ""
    
    demo_model_serving
    echo ""
    
    demo_resource_optimization
    echo ""
    
    demo_cost_analysis
    echo ""
    
    demo_cleanup
    echo ""
    
    # Summary
    print_colored $GREEN "✨ Demo Summary"
    echo "================="
    echo "Demonstrated real-world capabilities:"
    echo "1. Multi-team Development Environment"
    echo "   - Isolated resources for each developer"
    echo "   - Flexible resource allocation"
    echo ""
    echo "2. Production Model Serving"
    echo "   - Multiple models in production"
    echo "   - Resource isolation and guarantees"
    echo ""
    echo "3. Dynamic Resource Optimization"
    echo "   - Real-time scaling"
    echo "   - Load-based resource allocation"
    echo ""
    echo "4. Cost Efficiency"
    echo "   - Significant cost savings"
    echo "   - Better resource utilization"
    echo ""
    echo "5. Resource Management"
    echo "   - Easy cleanup and reallocation"
    echo "   - System monitoring and health checks"
}

# Run the demo
main
