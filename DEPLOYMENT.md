# Trilix Deployment Guide

## 🚀 Quick Start (Field Tested)

### 1. Prerequisites
- **AWS CLI** configured (`aws sso login`)
- **Docker Desktop** running
- **kubectl** installed and authenticated

### 2. Update Configuration
**Secrets (`k8s/02-secrets.yaml`)**:
⚠️ **CRITICAL**: Ensure the encryption key is named `API_KEY_ENCRYPTION_KEY`.
```yaml
stringData:
  POSTGRES_PASSWORD: "secure_password"
  RABBITMQ_DEFAULT_PASS: "secure_password"
  CLERK_SECRET_KEY: "sk_test_..."
  CLERK_PUBLISHABLE_KEY: "pk_test_..."
  API_KEY_ENCRYPTION_KEY: "32-character-random-string" # MUST match this name
```

### 3. Build & Push (Multi-Arch)
This script builds for **both** `linux/amd64` and `linux/arm64` to support any EKS node type (Intel or Graviton/Apple Silicon dev machines).

```bash
# Replace with your Account ID and Region
./build-and-push.sh 996894428841 us-east-1
```
*Note: If you are on a Mac, this uses `docker buildx` to ensure the image runs on EKS.*

### 4. Deploy to Kubernetes
```bash
./deploy.sh apply
```

### 5. Access the Application
Get the LoadBalancer URL:
```bash
kubectl get svc mcp-server -n trilix
```
Open the `EXTERNAL-IP` in your browser.
- **Frontend**: `http://<LOADBALANCER_URL>/workspaces.html`

---

## 🛠️ Troubleshooting

### 🛑 `CrashLoopBackOff` on RabbitMQ
**Cause**: Liveness probe timeout (default 5s is too short for some nodes).
**Fix**: Increase `timeoutSeconds` in `k8s/04-rabbitmq.yaml`:
```yaml
livenessProbe:
  timeoutSeconds: 30
readinessProbe:
  timeoutSeconds: 30
```

### 🛑 `CrashLoopBackOff` on Confluence/Jira
**Cause**: Missing or incorrectly named encryption key secret.
**Fix**: Ensure `k8s/02-secrets.yaml` has `API_KEY_ENCRYPTION_KEY` (not just `ENCRYPTION_KEY`).
```bash
kubectl apply -f k8s/02-secrets.yaml
kubectl rollout restart deployment confluence-service jira-service -n trilix
```

### 🛑 `ImagePullBackOff` / `no match for platform`
**Cause**: Building an ARM64 image (on M1/M2/M3 Mac) and deploying to an AMD64 (Intel) EKS node.
**Fix**: Use the updated `build-and-push.sh` which enforces multi-arch build:
```bash
export DOCKER_DEFAULT_PLATFORM=linux/amd64,linux/arm64
./build-and-push.sh <account-id> <region>
```

### 🛑 `Unauthorized` / `ExpiredToken`
**Cause**: AWS SSO session expired.
**Fix**: Re-authenticate and force the profile for the build script:
```bash
aws sso login
AWS_PROFILE=AdministratorAccess-996894428841 ./build-and-push.sh ...
```

---

## 📂 Project Structure
- **`build-and-push.sh`**: Automates Docker build/tag/push with multi-arch support.
- **`deploy.sh`**: Helper to apply/delete Kubernetes manifests.
- **`k8s/`**: Contains all Deployment, Service, StatefulSet, and ConfigMap files.
- **`Dockerfile`**: Unified Dockerfile for all 3 services (uses `build-arg SERVICE=...`).
