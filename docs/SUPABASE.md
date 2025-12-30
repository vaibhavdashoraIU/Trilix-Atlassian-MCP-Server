# Supabase Integration Guide

This guide explains how to migrate your Trilix Atlassian MCP Server from file-based storage (`workspaces.json`) to a central Supabase PostgreSQL database.

## 1. Supabase Setup

### Create a Database
1. Go to [Supabase Dashboard](https://database.new).
2. Create a new project.
3. Once the database is ready, go to **Project Settings** -> **Database**.
4. Copy the **Transaction** or **Session** connection string (URI format).

### Initialize Schema (Optional)
The server will automatically try to create the necessary tables on startup. However, if you prefer manual setup, run this SQL in the Supabase SQL Editor:

```sql
CREATE TABLE IF NOT EXISTS atlassian_credentials (
    user_id VARCHAR(255) NOT NULL,
    workspace_id VARCHAR(255) NOT NULL,
    workspace_name VARCHAR(255) NOT NULL,
    atlassian_url VARCHAR(500) NOT NULL,
    email VARCHAR(255) NOT NULL,
    api_token_encrypted TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, workspace_id)
);

CREATE INDEX IF NOT EXISTS idx_user_id ON atlassian_credentials(user_id);
```

## 2. Configuration (`.env`)

Update your `.env` file to point to Supabase.

1. **Comment out** `WORKSPACES_FILE`.
2. **Set** `DATABASE_URL` to your Supabase connection string.
3. **Set** `API_KEY_ENCRYPTION_KEY` to a 32-character secret.

```env
# WORKSPACES_FILE=.config/workspaces.json  <-- COMMENT THIS OUT

DATABASE_URL="postgres://postgres.xxxx:your-password@aws-0-us-east-1.pooler.supabase.com:6543/postgres?sslmode=disable"
API_KEY_ENCRYPTION_KEY="your-32-character-secret-key-123"
```

3. Restart your server.

## ⚠️ Important Note on Encryption
Your API tokens are encrypted *before* they leave your server using the `API_KEY_ENCRYPTION_KEY`. If you lose this key or change it, you will lose access to your stored workspaces because the server won't be able to decrypt the tokens. **Keep this key safe!**
