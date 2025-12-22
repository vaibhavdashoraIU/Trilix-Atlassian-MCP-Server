#!/bin/bash
# Build and push all Docker images to AWS ECR
# Usage: ./build-and-push.sh <aws-account-id> <region>

set -e

AWS_ACCOUNT_ID=${1:-"YOUR_ACCOUNT_ID"}
AWS_REGION=${2:-"us-east-1"}
ECR_REGISTRY="${AWS_ACCOUNT_ID}.dkr.ecr.${AWS_REGION}.amazonaws.com"

echo "🔐 Logging into AWS ECR..."
aws ecr get-login-password --region ${AWS_REGION} | \
    docker login --username AWS --password-stdin ${ECR_REGISTRY}

# Services to build
SERVICES=("mcp-server" "confluence-service" "jira-service")

for SERVICE in "${SERVICES[@]}"; do
    echo ""
    echo "🏗️  Building ${SERVICE}..."
    docker buildx build \
        --platform linux/amd64,linux/arm64 \
        --push \
        --build-arg SERVICE=${SERVICE} \
        -t ${ECR_REGISTRY}/trilix/${SERVICE}:latest \
        -f Dockerfile .
    
    # Ensure ECR repository exists
    echo "🔍 Checking ECR repository for trilix/${SERVICE}..."
    aws ecr describe-repositories --repository-names "trilix/${SERVICE}" --region ${AWS_REGION} > /dev/null 2>&1 || {
        echo "🆕 Creating repository trilix/${SERVICE}..."
        aws ecr create-repository --repository-name "trilix/${SERVICE}" --region ${AWS_REGION}
    }
    
    echo "✅ ${SERVICE} pushed successfully"
done

echo ""
echo "🎉 All services built and pushed to ECR!"
echo "Next steps:"
echo "  1. Update k8s manifests with ECR image URLs"
echo "  2. kubectl apply -f k8s/"
