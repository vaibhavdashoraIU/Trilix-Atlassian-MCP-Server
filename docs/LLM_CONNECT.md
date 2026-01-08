# Connecting an LLM to the Trilix MCP Server (OAuth 2.1)

This document lists the steps a user must complete **before** connecting any LLM (ChatGPT, Claude Desktop, Cursor) to the Trilix MCP Server.

## 1) Prerequisites

- Public HTTPS endpoint for the MCP server (e.g. `https://trilix-eso.aws.providentiaworldwide.com`)
- OAuth is enabled on the server (issuer, audience, signing key, DB, Redis)
- Clerk is configured (publishable + secret keys)
- MCP server is running and reachable

## 2) Verify OAuth Discovery

Open the discovery document:

```
GET https://trilix-eso.aws.providentiaworldwide.com/.well-known/oauth-authorization-server
```

You should see JSON containing:
- `authorization_endpoint`
- `token_endpoint`
- `jwks_uri`
- `registration_endpoint`
- `code_challenge_methods_supported`

If this is missing or 404s, OAuth is not initialized.

## 3) Generate the OAuth Signing Key

Generate an RSA private key (PEM):

```bash
openssl genrsa -out oauth-private-key.pem 2048
```

Kubernetes secret (recommended):

```bash
kubectl create secret generic trilix-oauth-key \
  -n trilix \
  --from-file=oauth-private-key.pem=/path/to/oauth-private-key.pem \
  --dry-run=client -o yaml | kubectl apply -f -
```

Ensure the deployment mounts the key via `OAUTH_PRIVATE_KEY_PATH` (already set in `k8s/01-namespace-configmap.yaml`).

## 3) Generate a DCR Admin Token

Create a secure token (used only to register clients):

```
openssl rand -hex 32
```

Set it on the server as:
```
OAUTH_DCR_ACCESS_TOKEN=<your-generated-token>
```

Kubernetes secret example:

```bash
kubectl apply -f k8s/02-secrets.yaml || update secret manager file on aws
```

## 4) Register an OAuth Client

Register a client for the LLM. For ChatGPT, use wildcard redirect URIs (so GPT IDs don’t break):

```bash
curl -X POST https://<your-domain>/oauth/register \
  -H "Authorization: Bearer <OAUTH_DCR_ACCESS_TOKEN>" \
  -H "Content-Type: application/json" \
  -d '{
    "redirect_uris": [
      "https://chat.openai.com/aip/g-*/oauth/callback",
      "https://chatgpt.com/aip/g-*/oauth/callback"
    ],
    "client_name": "ChatGPT",
    "token_endpoint_auth_method": "client_secret_post"
  }'
```

Save the response:
- `client_id`
- `client_secret` (only returned once)

For other LLMs (Claude Desktop, Cursor), use their specific redirect URI and select the appropriate `token_endpoint_auth_method`.

### Claude 

1) Find Claude redirect URI in its MCP connection settings UI.
2) Register a client with that exact redirect URI:

```bash
curl -X POST https://<your-domain>/oauth/register \
  -H "Authorization: Bearer <OAUTH_DCR_ACCESS_TOKEN>" \
  -H "Content-Type: application/json" \
  -d '{
    "redirect_uris": ["<CLAUDE_REDIRECT_URI>"],
    "client_name": "Claude Web",
    "token_endpoint_auth_method": "none"
  }'
```

3) Configure Claude Desktop:
- **Authorization URL**: `https://<your-domain>/oauth/authorize`
- **Token URL**: `https://<your-domain>/oauth/token`
- **Client ID**: from the registration response
- **Client Secret**: leave empty
- **Scope**: `mcp:tools mcp:resource`

## 5) Configure the LLM OAuth Settings

Use these values in the LLM configuration UI:

- **Authorization URL**: `https://<your-domain>/oauth/authorize`
- **Token URL**: `https://<your-domain>/oauth/token`
- **Client ID**: from step 4
- **Client Secret**: from step 4 (if `client_secret_post` was used)
- **Scope**: `mcp:tools mcp:resource`

> Important: Do **not** use `/.well-known/oauth-authorization-server` as the authorization URL.

## 6) First Sign-In (User)

When the user clicks “Sign in” in the LLM:

1. They will be redirected to `/oauth/authorize`
2. Clerk login completes on that page
3. The server issues an auth code
4. The LLM exchanges the code at `/oauth/token`

If the user is sent to the MCP home page and not back to the LLM, Clerk is redirecting incorrectly (check Clerk dashboard “After sign-in URL” and the OAuth authorize page).

## 7) Verify MCP Access

Once connected, test:

```
GET /api/workspaces
Authorization: Bearer <access_token>
```

If this fails:
- Check `/oauth/token` logs
- Verify `client_id` + `client_secret`
- Ensure redirect URI matches the LLM

## 8) Workspace Connection (Atlassian)

LLM calls to Confluence/Jira require valid Atlassian credentials stored for the logged-in user. If you see 403 from Atlassian:
- Generate a new Atlassian API token
- Reconnect the workspace with correct email + token

## Security Notes

- Never commit private keys or `.env` files
- Keep `OAUTH_DCR_ACCESS_TOKEN` secret
- Use HTTPS only in production
- Rotate `client_secret` if leaked
