$ErrorActionPreference = "SilentlyContinue"

Write-Host "🛑 Stopping Trilix Services..." -ForegroundColor Yellow

# Kill process on port 3000
Write-Host "🧹 Cleaning up port 3000..." -ForegroundColor Yellow

$tcp = Get-NetTCPConnection -LocalPort 3000
if ($tcp) {
    Stop-Process -Id $tcp.OwningProcess -Force
    Write-Host "   Killed process on port 3000" -ForegroundColor Green
}

# Kill mcp-server.exe if running
$proc = Get-Process -Name "mcp-server"
if ($proc) {
    Stop-Process -Name "mcp-server" -Force
    Write-Host "   Killed mcp-server.exe" -ForegroundColor Green
}

# Kill microservices (go run produces main.exe usually)
Get-Process -Name "main" -ErrorAction SilentlyContinue | Stop-Process -Force
Get-Process -Name "go" -ErrorAction SilentlyContinue | Stop-Process -Force
Write-Host "   Killed Go processes" -ForegroundColor Green

Write-Host "✅ All services stopped." -ForegroundColor Green
