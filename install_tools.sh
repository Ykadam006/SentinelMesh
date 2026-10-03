#!/bin/bash

echo "Starting SentinelMesh Setup for macOS..."

# Check if Homebrew is installed
if ! command -v brew &> /dev/null
then
    echo "Homebrew could not be found. Please install it first:"
    echo '/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"'
    exit 1
fi

echo "Installing required tools..."
brew install go
brew install protobuf
brew install protoc-gen-go
brew install protoc-gen-go-grpc

# Infrastructure tools
brew install kind
brew install terraform
brew install helm
brew install argocd

# Create local Kubernetes cluster
echo "Creating kind cluster 'sentinelmesh-cluster'..."
kind create cluster --name sentinelmesh-cluster || echo "Cluster might already exist"

# Init terraform
echo "Initializing Terraform..."
cd infra/terraform
terraform init
cd ../..

echo "Setup complete! You can now apply terraform or start the docker-compose stack."
