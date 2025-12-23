$ErrorActionPreference = "Stop"
$PORT = 3000

Write-Host "🚀 Starting Trilix Services..." -ForegroundColor Green

# Kill existing process on port 3000
Write-Host "🧹 Cleaning up port $PORT..." -ForegroundColor Yellow
try {
    $tcp = Get-NetTCPConnection -LocalPort $PORT -ErrorAction SilentlyContinue
    if ($tcp) {
        $proc = Get-Process -Id $tcp.OwningProcess -ErrorAction SilentlyContinue
        if ($proc) {
            Stop-Process -Id $proc.Id -Force
            Write-Host "   Killed process $($proc.Id)" -ForegroundColor Gray
        }
    }
} catch {
    # Ignore errors if no process found
}

# Start Microservices
Write-Host "🚀 Starting Confluence Service..." -ForegroundColor Cyan
Start-Process -FilePath "go" -ArgumentList "run", "main.go" -WorkingDirectory "cmd/confluence-service" -WindowStyle Hidden -RedirectStandardOutput "../../confluence-service.log" -RedirectStandardError "../../confluence-service.log"

Write-Host "🚀 Starting Jira Service..." -ForegroundColor Cyan
Start-Process -FilePath "go" -ArgumentList "run", "main.go" -WorkingDirectory "cmd/jira-service" -WindowStyle Hidden -RedirectStandardOutput "../../jira-service.log" -RedirectStandardError "../../jira-service.log"

Start-Sleep -Seconds 3

# Build the server
Write-Host "🔨 Building Go Server..." -ForegroundColor Cyan
Set-Location "cmd/mcp-server"

# Build with .exe extension for Windows
go build -o mcp-server.exe

if ($LASTEXITCODE -eq 0) {
    Write-Host "✅ Build successful!" -ForegroundColor Green
    Write-Host "🌐 Starting server on http://localhost:$PORT" -ForegroundColor Cyan
    Write-Host "   - Frontend: http://localhost:$PORT/trilix-workspaces.html"
    Write-Host "   - Test Client: http://localhost:$PORT/docs/test-client.html"
    Write-Host "   - API: http://localhost:$PORT/api/workspaces"
    Write-Host "   - SSE: http://localhost:$PORT/sse"
    Write-Host ""
    Write-Host "Logs:" -ForegroundColor Yellow
    
    # Run the executable
    .\mcp-server.exe
} else {
    Write-Host "❌ Build failed!" -ForegroundColor Red
    exit 1
}
