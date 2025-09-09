#!/bin/bash

# GCP VM Creation Script for AI Agent Demo Testing
# Supports both CPU-only and GPU-enabled instances

set -euo pipefail

echo "🚀 GCP VM Creation for AI Agent Demo"
echo "==================================="

# Configuration variables
PROJECT_ID=""
VM_NAME="ai-agent-demo"
ZONE="us-central1-c"
MACHINE_TYPE=""
GPU_TYPE=""
DISK_SIZE="50GB"
IMAGE_FAMILY="ubuntu-2204-lts"
IMAGE_PROJECT="ubuntu-os-cloud"

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

# Function to show usage
show_usage() {
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  -n, --name NAME          VM instance name (default: ai-agent-demo)"
    echo "  -p, --project PROJECT    GCP project ID (required)"
    echo "  -z, --zone ZONE          GCP zone (default: us-central1-c)"
    echo "  -t, --type TYPE          VM type: cpu|gpu-t4|gpu-l4|gpu-a100 (default: cpu)"
    echo "  -d, --disk-size SIZE     Boot disk size (default: 50GB)"
    echo "  --spot                   Use spot instances for cost savings"
    echo "  --help                   Show this help message"
    echo ""
    echo "Examples:"
    echo "  $0 -p my-project -n ai-demo -t cpu"
    echo "  $0 -p my-project -n ai-gpu-demo -t gpu-t4 --spot"
    echo "  $0 -p my-project -n ai-high-perf -t gpu-a100 -z us-central1-a"
}

# Parse command line arguments
USE_SPOT=""
while [[ $# -gt 0 ]]; do
    case $1 in
        -n|--name)
            VM_NAME="$2"
            shift 2
            ;;
        -p|--project)
            PROJECT_ID="$2"
            shift 2
            ;;
        -z|--zone)
            ZONE="$2"
            shift 2
            ;;
        -t|--type)
            VM_TYPE="$2"
            shift 2
            ;;
        -d|--disk-size)
            DISK_SIZE="$2"
            shift 2
            ;;
        --spot)
            USE_SPOT="--provisioning-model=SPOT"
            shift
            ;;
        --help)
            show_usage
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            show_usage
            exit 1
            ;;
    esac
done

# Validate required parameters
if [ -z "$PROJECT_ID" ]; then
    print_colored $RED "❌ Error: Project ID is required. Use -p or --project"
    show_usage
    exit 1
fi

# Set default VM type if not specified
if [ -z "${VM_TYPE:-}" ]; then
    VM_TYPE="cpu"
    print_colored $YELLOW "⚠️  No VM type specified, using default: cpu"
fi

# Configure machine type and GPU based on VM_TYPE
case "$VM_TYPE" in
    "cpu")
        MACHINE_TYPE="n2-standard-4"
        GPU_ARGS=""
        print_colored $BLUE "🖥️  CPU Instance: 4 vCPUs, 16GB RAM"
        ;;
    "gpu-t4")
        MACHINE_TYPE="n1-standard-4"
        GPU_ARGS="--accelerator type=nvidia-tesla-t4,count=1 --maintenance-policy TERMINATE"
        print_colored $BLUE "🎮 GPU T4 Instance: 4 vCPUs, 15GB RAM, 1x NVIDIA T4 (16GB VRAM)"
        print_colored $BLUE "💰 Cost: ~$0.54/hour + $0.35/hour GPU = $0.89/hour"
        ;;
    "gpu-l4")
        MACHINE_TYPE="g2-standard-4"
        GPU_ARGS="--maintenance-policy TERMINATE"
        print_colored $BLUE "🚀 GPU L4 Instance: 4 vCPUs, 16GB RAM, 1x NVIDIA L4 (24GB VRAM)"
        print_colored $BLUE "💰 Cost: ~$0.70/hour (L4 included in G2 machine type)"
        print_colored $BLUE "⚡ 4x faster than T4, better price/performance"
        ;;
    "gpu-a100")
        MACHINE_TYPE="a2-highgpu-1g"
        GPU_ARGS="--maintenance-policy TERMINATE"
        print_colored $BLUE "🔥 GPU A100 Instance: 12 vCPUs, 85GB RAM, 1x NVIDIA A100 (40GB VRAM)"
        print_colored $BLUE "💰 Cost: ~$3.67/hour (high performance)"
        ;;
    *)
        print_colored $RED "❌ Invalid VM type: $VM_TYPE"
        print_colored $YELLOW "Valid types: cpu, gpu-t4, gpu-l4, gpu-a100"
        exit 1
        ;;
esac

# Show configuration summary
print_colored $GREEN "📋 VM Configuration Summary"
echo "=================================="
echo "Project ID: $PROJECT_ID"
echo "VM Name: $VM_NAME"
echo "Zone: $ZONE"
echo "Machine Type: $MACHINE_TYPE"
echo "Boot Disk Size: $DISK_SIZE"
echo "Image: $IMAGE_FAMILY"
if [ -n "$GPU_ARGS" ]; then
    echo "GPU: Enabled"
fi
if [ -n "$USE_SPOT" ]; then
    echo "Spot Instance: Yes (up to 80% cost savings)"
fi
echo ""

# Ask for confirmation
read -p "Do you want to create this VM? (y/N): " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    print_colored $YELLOW "❌ VM creation cancelled"
    exit 0
fi

# Set the project
print_colored $BLUE "🔧 Setting GCP project..."
gcloud config set project $PROJECT_ID

# Check if VM already exists
if gcloud compute instances describe $VM_NAME --zone=$ZONE &>/dev/null; then
    print_colored $YELLOW "⚠️  VM '$VM_NAME' already exists in zone '$ZONE'"
    read -p "Do you want to delete it and create a new one? (y/N): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        print_colored $BLUE "🗑️  Deleting existing VM..."
        gcloud compute instances delete $VM_NAME --zone=$ZONE --quiet
    else
        print_colored $YELLOW "❌ VM creation cancelled"
        exit 0
    fi
fi

# Create the VM
print_colored $BLUE "🚀 Creating VM instance..."

# Build the gcloud command
GCLOUD_CMD="gcloud compute instances create $VM_NAME \
  --project=$PROJECT_ID \
  --zone=$ZONE \
  --machine-type=$MACHINE_TYPE \
  --boot-disk-size=$DISK_SIZE \
  --boot-disk-type=pd-standard \
  --boot-disk-device-name=$VM_NAME \
  --image-family=$IMAGE_FAMILY \
  --image-project=$IMAGE_PROJECT \
  --tags=ai-agent-vm,http-server,https-server"

# Add GPU args if specified
if [ -n "$GPU_ARGS" ]; then
    GCLOUD_CMD="$GCLOUD_CMD $GPU_ARGS"
fi

# Add spot provisioning if specified
if [ -n "$USE_SPOT" ]; then
    GCLOUD_CMD="$GCLOUD_CMD $USE_SPOT"
fi

# Add startup script for automatic setup
STARTUP_SCRIPT='#!/bin/bash
apt-get update
apt-get install -y curl wget git htop
echo "AI Agent VM setup completed" > /tmp/startup-complete
'

GCLOUD_CMD="$GCLOUD_CMD --metadata startup-script='$STARTUP_SCRIPT'"

# Execute the command
eval $GCLOUD_CMD

if [ $? -eq 0 ]; then
    print_colored $GREEN "✅ VM created successfully!"
    
    # Wait for VM to be ready
    print_colored $BLUE "⏳ Waiting for VM to be ready..."
    sleep 30
    
    # Get VM details
    VM_IP=$(gcloud compute instances describe $VM_NAME --zone=$ZONE --format='get(networkInterfaces[0].accessConfigs[0].natIP)')
    VM_INTERNAL_IP=$(gcloud compute instances describe $VM_NAME --zone=$ZONE --format='get(networkInterfaces[0].networkIP)')
    
    print_colored $GREEN "🎉 VM Setup Complete!"
    echo "=================================="
    echo "VM Name: $VM_NAME"
    echo "Zone: $ZONE"
    echo "External IP: $VM_IP"
    echo "Internal IP: $VM_INTERNAL_IP"
    echo "Machine Type: $MACHINE_TYPE"
    echo ""
    
    # Create firewall rules for AI agent service
    print_colored $BLUE "🔒 Creating firewall rules for AI Agent service..."
    
    # Allow port 3000 for AI agent service
    if ! gcloud compute firewall-rules describe allow-ai-agent-service &>/dev/null; then
        gcloud compute firewall-rules create allow-ai-agent-service \
            --allow tcp:3000 \
            --source-ranges 0.0.0.0/0 \
            --target-tags ai-agent-vm \
            --description "Allow AI Agent service on port 3000"
        print_colored $GREEN "✅ Firewall rule created for port 3000"
    else
        print_colored $YELLOW "⚠️  Firewall rule already exists"
    fi
    
    # SSH connection command
    print_colored $GREEN "🔗 Connection Commands:"
    echo "=================================="
    echo "SSH to VM:"
    echo "  gcloud compute ssh $VM_NAME --zone=$ZONE"
    echo ""
    echo "Copy files to VM:"
    echo "  gcloud compute scp local-file.txt $VM_NAME:~/ --zone=$ZONE"
    echo ""
    echo "Port forwarding (for local testing):"
    echo "  gcloud compute ssh $VM_NAME --zone=$ZONE --ssh-flag=\"-L 3000:localhost:3000\""
    echo ""
    
    # AI Agent deployment commands
    print_colored $GREEN "🤖 AI Agent Deployment Commands:"
    echo "=================================="
    echo "1. SSH into the VM:"
    echo "   gcloud compute ssh $VM_NAME --zone=$ZONE"
    echo ""
    echo "2. Deploy AI Agent VM:"
    echo "   curl -fsSL https://raw.githubusercontent.com/your-repo/docker/claude-code-vm/deploy-gcp.sh | bash"
    echo ""
    echo "3. Test the service:"
    echo "   curl http://$VM_IP:3000/health"
    echo ""
    
    # Cost estimation
    print_colored $YELLOW "💰 Cost Estimation:"
    echo "=================================="
    case "$VM_TYPE" in
        "cpu")
            echo "Estimated cost: ~$0.19/hour (n2-standard-4)"
            ;;
        "gpu-t4")
            echo "Estimated cost: ~$0.89/hour (n1-standard-4 + T4 GPU)"
            ;;
        "gpu-l4")
            echo "Estimated cost: ~$0.70/hour (g2-standard-4 with L4)"
            ;;
        "gpu-a100")
            echo "Estimated cost: ~$3.67/hour (a2-highgpu-1g with A100)"
            ;;
    esac
    
    if [ -n "$USE_SPOT" ]; then
        echo "Spot instance: Up to 80% cost savings"
    fi
    
    echo ""
    print_colored $YELLOW "⚠️  Remember to delete the VM when done to avoid charges:"
    echo "  gcloud compute instances delete $VM_NAME --zone=$ZONE"
    
else
    print_colored $RED "❌ Failed to create VM"
    exit 1
fi