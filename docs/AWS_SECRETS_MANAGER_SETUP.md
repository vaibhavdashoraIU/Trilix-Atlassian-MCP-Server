# AWS Secrets Manager Setup (EKS + IRSA)

Use these steps to let the services load secrets from AWS Secrets Manager via IRSA.

## 1) Create the secret
1. AWS Console → Secrets Manager → **Store a new secret**.
2. Secret type: **Other type of secret**.
3. Secret value: **Plaintext**. Paste your JSON (example):
   ```json
   {
     "RABBITMQ_HOST": "localhost",
     "RABBITMQ_VHOST": "trilix",
     "RABBITMQ_USER": "trilix",
     "RABBITMQ_PASSWORD": "secret",
     "DATABASE_URL": "postgresql://user:pass@aws-1-eu-west-1.pooler.supabase.com:5432/postgres",
     "CLERK_API_URL": "https://api.clerk.com",
     "CLERK_PUBLISHABLE_KEY": "pk_test_....",
     "CLERK_SECRET_KEY": "sk_test_....",
     "API_KEY_ENCRYPTION_KEY": "abcd...."
   }
   ```
4. Name the secret (e.g., `prod/trilix/mcp`). Leave resource permissions and replication empty unless needed. Store it.

## 2) Create IAM role for IRSA
1. IAM → Roles → **Create role** → **Web identity**.
2. Identity provider: choose the EKS OIDC for the cluster.
3. Audience: `sts.amazonaws.com`.
4. Add condition:
   - Key: `sub`
   - Condition: `StringEquals`
   - Value: `system:serviceaccount:trilix:trilix-runner` (replace if you use another SA name).
5. Permissions: create/attach a custom policy:
   ```json
   {
     "Version": "2012-10-17",
     "Statement": [
       {
         "Effect": "Allow",
         "Action": "secretsmanager:GetSecretValue",
         "Resource": "arn:aws:secretsmanager:<region>:<account-id>:secret:prod/trilix/mcp-*"
       }
     ]
   }
   ```
6. Name the role (e.g., `trilix-secrets-role`) and create it.

## 3) Create/annotate the Kubernetes ServiceAccount
```bash
cat <<'EOF' | kubectl apply -f -
apiVersion: v1
kind: ServiceAccount
metadata:
  name: trilix-runner
  namespace: trilix
  annotations:
    eks.amazonaws.com/role-arn: arn:aws:iam::<account-id>:role/trilix-secrets-role
EOF
```

## 4) Point deployments to the ServiceAccount
```bash
kubectl patch deploy confluence-service -n trilix -p '{"spec":{"template":{"spec":{"serviceAccountName":"trilix-runner"}}}}'
kubectl patch deploy jira-service -n trilix -p '{"spec":{"template":{"spec":{"serviceAccountName":"trilix-runner"}}}}'
kubectl patch deploy mcp-server -n trilix -p '{"spec":{"template":{"spec":{"serviceAccountName":"trilix-runner"}}}}'
```

## 5) Set env vars for Secrets Manager
Ensure each deployment (or a shared K8s manifest) has:
```yaml
env:
  - name: AWS_SECRETS_MANAGER_SECRET_ID
    value: prod/trilix/mcp
  - name: AWS_SECRETS_MANAGER_REGION
    value: <region>
  # optional: overwrite existing envs with secret values
  # - name: AWS_SECRETS_MANAGER_OVERWRITE
  #   value: "true"
```
Redeploy or restart pods after updating envs.

### Quick add via kubectl (existing deployments)
If you just need to inject the env vars into running deployments:
```bash
kubectl set env deploy/{confluence-service,jira-service,mcp-server} -n trilix \
  AWS_SECRETS_MANAGER_SECRET_ID=prod/trilix/mcp \
  AWS_SECRETS_MANAGER_REGION=<region>

# Optional overwrite flag
kubectl set env deploy/{confluence-service,jira-service,mcp-server} -n trilix \
  AWS_SECRETS_MANAGER_OVERWRITE=true

# Restart to pick up the new env vars
kubectl rollout restart deploy confluence-service jira-service mcp-server -n trilix
kubectl rollout status deploy confluence-service jira-service mcp-server -n trilix
```

## 6) Verify
- Pods should log that they loaded env vars from Secrets Manager (see `internal/config/env.go` behavior).
- You can also `kubectl exec` and run `aws sts get-caller-identity` to confirm the IRSA role, then `aws secretsmanager get-secret-value --secret-id prod/trilix/mcp` (if AWS CLI is present) to verify access.
