# Quick Fix: kubectl Authentication Issue

## Problem
You're getting: `the server has asked for the client to provide credentials`

## Solution

### Step 1: Refresh AWS SSO Credentials

```bash
# Log in to AWS SSO
aws sso login

# Update kubeconfig again
aws eks update-kubeconfig --name pwweso-eks-mvp --region us-east-1

# Test
kubectl get nodes
```

### Step 2: If Still Not Working, Check IAM Role

Your AWS SSO role needs to be added to the EKS cluster's aws-auth ConfigMap. Ask your AWS admin to run:

```bash
eksctl create iamidentitymapping \
  --cluster pwweso-eks-mvp \
  --region us-east-1 \
  --arn arn:aws:iam::996894428841:role/AWSReservedSSO_PowerUserAccess_b5ef8883f0eb5f3e \
  --group system:masters \
  --username vaibhav.dashora
```

---

## What We've Completed So Far

✅ **Step 1-3**: Secrets configured  
✅ **Step 4**: Using existing cluster `pwweso-eks-mvp`  
✅ **Step 5**: ECR repositories created:
- `996894428841.dkr.ecr.us-east-1.amazonaws.com/trilix/mcp-server`
- `996894428841.dkr.ecr.us-east-1.amazonaws.com/trilix/confluence-service`
- `996894428841.dkr.ecr.us-east-1.amazonaws.com/trilix/jira-service`

✅ **Step 7**: K8s manifests updated with ECR URLs

---

## Next Steps (After Fixing kubectl)

### Step 6: Build and Push Docker Images

```bash
# This will take 5-10 minutes
./build-and-push.sh 996894428841 us-east-1
```

### Step 8: Deploy to Kubernetes

```bash
./deploy.sh apply
```

### Step 9: Get LoadBalancer URL

```bash
kubectl get svc mcp-server -n trilix
```

---

## Alternative: Deploy Without kubectl Access

If you can't fix kubectl auth right now, you can still:

1. **Build and push images** (doesn't need kubectl):
   ```bash
   ./build-and-push.sh 996894428841 us-east-1
   ```

2. **Ask your AWS admin to deploy** using the `k8s/` manifests

3. **Or wait until kubectl access is fixed**, then deploy yourself
