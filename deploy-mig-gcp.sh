#!/bin/bash

# MIG Server Deployment Script for GCP VM
# Installs NVIDIA drivers, CUDA, and deploys the MIG server

set -euo pipefail

echo "🚀 MIG Server Deployment for GCP VM"
echo "==================================="

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

# Check if running as root
if [ "$EUID" -eq 0 ]; then
    print_colored $RED "❌ Please don't run this script as root"
    exit 1
fi

# Update system
print_colored $BLUE "📦 Updating system packages..."
sudo apt-get update
sudo apt-get upgrade -y

# Install basic dependencies
print_colored $BLUE "🔧 Installing basic dependencies..."
sudo apt-get install -y \
    curl \
    wget \
    git \
    htop \
    build-essential \
    software-properties-common \
    apt-transport-https \
    ca-certificates \
    gnupg \
    lsb-release

# Install Go
print_colored $BLUE "🐹 Installing Go..."
GO_VERSION="1.21.5"
wget https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go${GO_VERSION}.linux-amd64.tar.gz
rm go${GO_VERSION}.linux-amd64.tar.gz

# Add Go to PATH
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
export PATH=$PATH:/usr/local/go/bin

# Install Docker
print_colored $BLUE "🐳 Installing Docker..."
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /usr/share/keyrings/docker-archive-keyring.gpg
echo "deb [arch=amd64 signed-by=/usr/share/keyrings/docker-archive-keyring.gpg] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null
sudo apt-get update
sudo apt-get install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin
sudo usermod -aG docker $USER

# Install NVIDIA drivers and CUDA
print_colored $BLUE "🎮 Installing NVIDIA drivers and CUDA..."

# Add NVIDIA package repositories
wget https://developer.download.nvidia.com/compute/cuda/repos/ubuntu2204/x86_64/cuda-keyring_1.0-1_all.deb
sudo dpkg -i cuda-keyring_1.0-1_all.deb
sudo apt-get update

# Install NVIDIA drivers
sudo apt-get install -y nvidia-driver-535
sudo apt-get install -y cuda-toolkit-12-2

# Install NVIDIA Container Toolkit
print_colored $BLUE "📦 Installing NVIDIA Container Toolkit..."
distribution=$(. /etc/os-release;echo $ID$VERSION_ID)
curl -s -L https://nvidia.github.io/nvidia-docker/gpgkey | sudo apt-key add -
curl -s -L https://nvidia.github.io/nvidia-docker/$distribution/nvidia-docker.list | sudo tee /etc/apt/sources.list.d/nvidia-docker.list
sudo apt-get update
sudo apt-get install -y nvidia-docker2
sudo systemctl restart docker

# Clone the repository
print_colored $BLUE "📁 Cloning MIG server repository..."
if [ -d "hypercore" ]; then
    print_colored $YELLOW "⚠️  Repository already exists, updating..."
    cd hypercore
    git pull
else
    git clone https://github.com/vistara/hypercore.git
    cd hypercore
fi

# Build the MIG server
print_colored $BLUE "🔨 Building MIG server..."
go mod tidy
go build -o mig-server ./cmd/mig-server

# Create systemd service
print_colored $BLUE "⚙️  Creating systemd service..."
sudo tee /etc/systemd/system/mig-server.service > /dev/null <<EOF
[Unit]
Description=MIG Server
After=network.target

[Service]
Type=simple
User=$USER
WorkingDirectory=/home/$USER/hypercore
ExecStart=/home/$USER/hypercore/mig-server
Restart=always
RestartSec=5
Environment=PORT=8080
Environment=LOG_LEVEL=info

[Install]
WantedBy=multi-user.target
EOF

# Enable and start the service
sudo systemctl daemon-reload
sudo systemctl enable mig-server
sudo systemctl start mig-server

# Wait for service to start
print_colored $BLUE "⏳ Waiting for MIG server to start..."
sleep 10

# Check service status
if sudo systemctl is-active --quiet mig-server; then
    print_colored $GREEN "✅ MIG server is running!"
else
    print_colored $RED "❌ MIG server failed to start"
    sudo systemctl status mig-server
    exit 1
fi

# Test the API
print_colored $BLUE "🧪 Testing MIG server API..."
if curl -f http://localhost:8080/api/v1/health > /dev/null 2>&1; then
    print_colored $GREEN "✅ MIG server API is responding!"
else
    print_colored $YELLOW "⚠️  MIG server API test failed, but service is running"
fi

# Get external IP
EXTERNAL_IP=$(curl -s http://metadata.google.internal/computeMetadata/v1/instance/network-interfaces/0/access-configs/0/external-ip -H "Metadata-Flavor: Google")

print_colored $GREEN "🎉 MIG Server Deployment Complete!"
echo "=================================="
echo "MIG Server Status: Running"
echo "Local API: http://localhost:8080"
echo "External API: http://$EXTERNAL_IP:8080"
echo ""
echo "Health Check:"
echo "  curl http://localhost:8080/api/v1/health"
echo ""
echo "Service Management:"
echo "  sudo systemctl status mig-server"
echo "  sudo systemctl restart mig-server"
echo "  sudo systemctl stop mig-server"
echo ""
echo "Logs:"
echo "  sudo journalctl -u mig-server -f"
echo ""

# Test GPU detection
print_colored $BLUE "🎮 Testing GPU detection..."
if command -v nvidia-smi &> /dev/null; then
    nvidia-smi
    print_colored $GREEN "✅ NVIDIA drivers are working!"
else
    print_colored $YELLOW "⚠️  NVIDIA drivers not detected. Please reboot the VM."
fi

print_colored $YELLOW "⚠️  Important: Please reboot the VM to ensure all drivers are loaded properly:"
echo "  sudo reboot"