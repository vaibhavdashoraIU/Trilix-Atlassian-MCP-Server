# n8n Cloud Integration Setup

## ⚠️ Important: You're Using n8n Cloud

n8n Cloud (`pww.app.n8n.cloud`) **cannot access files on your local machine**. You need to expose your MCP server to the internet.

## 🌐 Solution: Use ngrok to Expose Local Server

### Step 1: Install ngrok

**Option A: Using Homebrew (macOS)**
```bash
brew install ngrok
```

**Option B: Download from ngrok.com**
1. Go to: https://ngrok.com/download
2. Download and install ngrok
3. Sign up for free account at: https://dashboard.ngrok.com/signup

### Step 2: Authenticate ngrok (First Time Only)
```bash
ngrok config add-authtoken YOUR_AUTH_TOKEN
```
Get your auth token from: https://dashboard.ngrok.com/get-started/your-authtoken

### Step 3: Start ngrok Tunnel
```bash
# In a new terminal window
ngrok http 3001
```

You'll see output like:
```
Forwarding  https://abc123.ngrok.io -> http://localhost:3001
```

### Step 4: Use ngrok URL in n8n Cloud

In your n8n workflow:

**Endpoint:** `https://abc123.ngrok.io` (use YOUR ngrok URL)

**Example:**
```
https://1234-56-78-90-123.ngrok-free.app
```

## 🚀 Complete Setup Commands

```bash
# Terminal 1: Start backend services
cd /Users/vaibhavdashora/Desktop/IdeaUsher/Trilix-Atlassian-MCP-Server
./start-services.sh

# Terminal 2: Start ngrok tunnel
ngrok http 3001
```

## 📋 n8n Cloud Configuration

1. **Open your n8n workflow**
2. **Add MCP node**
3. **Configure:**
   - **Endpoint:** `https://YOUR-NGROK-URL.ngrok.io`
   - **Transport:** SSE (if available) or leave default
4. **Test Connection**

## 🧪 Test Your Setup

### Test 1: Check local server
```bash
curl http://localhost:3001/sse
```

### Test 2: Check ngrok tunnel
```bash
curl https://YOUR-NGROK-URL.ngrok.io/sse
```

### Test 3: Test from n8n Cloud
Use the ngrok URL in n8n and click "Test Connection"

## 💡 Alternative: Deploy to Cloud

If you don't want to use ngrok, deploy your MCP server to:
- **AWS EC2** / **DigitalOcean** / **Heroku**
- **Railway.app** (easiest)
- **Fly.io**

Then use the public URL in n8n Cloud.

## ⚠️ ngrok Free Tier Limitations

- URL changes every time you restart ngrok
- 40 connections/minute limit
- Session expires after 2 hours

**For production:** Use a paid ngrok plan or deploy to cloud.

## 🔒 Security Note

Your MCP server will be publicly accessible via ngrok. Consider adding:
- API key authentication
- IP whitelisting
- Rate limiting

## 📝 Quick Reference

```bash
# Start everything
./start-services.sh

# In new terminal, start ngrok
ngrok http 3001

# Copy the https URL from ngrok output
# Paste it in n8n Cloud as the endpoint
```

## ✅ Checklist

- [ ] Backend services running (`./start-services.sh`)
- [ ] ngrok installed
- [ ] ngrok authenticated
- [ ] ngrok tunnel running (`ngrok http 3001`)
- [ ] Copied ngrok HTTPS URL
- [ ] Used ngrok URL in n8n Cloud
- [ ] Tested connection in n8n

## 🆘 Troubleshooting

### ngrok shows "command not found"
Install ngrok: `brew install ngrok`

### ngrok shows "authentication required"
Run: `ngrok config add-authtoken YOUR_TOKEN`

### n8n still can't connect
1. Check ngrok is running: Look for "Forwarding" line
2. Test ngrok URL: `curl https://YOUR-URL.ngrok.io/sse`
3. Check backend services: `ps aux | grep "confluence-service"`

### ngrok URL keeps changing
This is normal for free tier. Update n8n each time you restart ngrok.
For permanent URL, upgrade to ngrok paid plan.
