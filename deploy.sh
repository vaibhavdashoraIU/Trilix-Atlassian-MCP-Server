#!/bin/bash
# Deploy Trilix to Kubernetes
# Usage: ./deploy.sh [apply|delete|status]

set -e

ACTION=${1:-apply}
NAMESPACE="trilix"

case $ACTION in
  apply)
    echo "🚀 Deploying Trilix to Kubernetes..."
    
    # Apply in order
    echo "📦 Creating namespace and config..."
    kubectl apply -f k8s/01-namespace-configmap.yaml
    
    echo "🔐 Creating secrets..."
    kubectl apply -f k8s/02-secrets.yaml
    
    echo "🗄️  Deploying PostgreSQL..."
    kubectl apply -f k8s/03-postgres.yaml
    
    echo "🐰 Deploying RabbitMQ..."
    kubectl apply -f k8s/04-rabbitmq.yaml
    
    echo "⏳ Waiting for dependencies to be ready..."
    kubectl wait --for=condition=ready pod -l app=postgres -n ${NAMESPACE} --timeout=300s || true
    kubectl wait --for=condition=ready pod -l app=rabbitmq -n ${NAMESPACE} --timeout=300s || true
    
    echo "🔧 Deploying services..."
    kubectl apply -f k8s/05-confluence-service.yaml
    kubectl apply -f k8s/06-jira-service.yaml
    kubectl apply -f k8s/07-mcp-server.yaml
    
    echo "🌐 Creating ALB Ingress..."
    kubectl apply -f k8s/08-mcp-ingress.yaml
    
    echo ""
    echo "✅ Deployment complete!"
    echo ""
    echo "📊 Check status with: ./deploy.sh status"
    echo "🌐 Get ALB URL with: kubectl get ingress mcp-ingress -n ${NAMESPACE}"
    ;;
    
  delete)
    echo "🗑️  Deleting Trilix from Kubernetes..."
    kubectl delete -f k8s/ --ignore-not-found=true
    echo "✅ Deletion complete!"
    ;;
    
  status)
    echo "📊 Trilix Deployment Status"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
    echo "🏷️  Namespace:"
    kubectl get namespace ${NAMESPACE} 2>/dev/null || echo "  Namespace not found"
    echo ""
    echo "📦 Pods:"
    kubectl get pods -n ${NAMESPACE} -o wide
    echo ""
    echo "🔌 Services:"
    kubectl get svc -n ${NAMESPACE}
    echo ""
    echo "💾 Persistent Volume Claims:"
    kubectl get pvc -n ${NAMESPACE}
    echo ""
    echo "🌐 Ingress (ALB) Status:"
    kubectl get ingress -n ${NAMESPACE}
    echo ""
    echo "🔗 ALB DNS Name:"
    kubectl get ingress mcp-ingress -n ${NAMESPACE} -o jsonpath='{.status.loadBalancer.ingress[0].hostname}' 2>/dev/null && echo "" || echo "  Not yet provisioned (Wait 2-3 mins)"
    ;;
    
  *)
    echo "Usage: $0 [apply|delete|status]"
    echo "  apply  - Deploy all resources"
    echo "  delete - Remove all resources"
    echo "  status - Show deployment status"
    exit 1
    ;;
esac
