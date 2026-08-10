# ============================================================
# start-remote.ps1 -- Arranca el backend apuntando a la BD remota
# Uso: .\start-remote.ps1  (desde la raiz del proyecto)
# ============================================================

$ErrorActionPreference = "Stop"
$Root      = $PSScriptRoot
$Backend   = Join-Path $Root "backend"
$EnvRemote = Join-Path $Root ".env.remote"
$EnvTemp   = Join-Path $Backend ".env"

# --- 0. Verificar que .env.remote existe ---
if (-not (Test-Path $EnvRemote)) {
    Write-Host ""
    Write-Host "  [ERROR] No se encontro el archivo .env.remote" -ForegroundColor Red
    Write-Host "  Crea el archivo .env.remote en la raiz del proyecto." -ForegroundColor Red
    Write-Host ""
    exit 1
}

# --- 1. Recordatorio del tunel SSH ---
Write-Host ""
Write-Host "========================================================" -ForegroundColor Cyan
Write-Host "  MODO REMOTO -- BD en servidor 109.199.104.87" -ForegroundColor Cyan
Write-Host "========================================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "  ANTES de continuar, asegurate de tener el tunel SSH" -ForegroundColor Yellow
Write-Host "  abierto en otra terminal (apuntando a PostgreSQL):" -ForegroundColor Yellow
Write-Host ""
Write-Host "    ssh -L 5435:postgres-db:5432 root@109.199.104.87" -ForegroundColor White
Write-Host "    (alternativa: ssh -L 5435:localhost:5432 root@109.199.104.87)" -ForegroundColor Gray
Write-Host "    Password: oso20759364" -ForegroundColor White
Write-Host ""

# --- 2. Verificar que el puerto 5435 responde ---
Write-Host "  Verificando tunel SSH en localhost:5435..." -ForegroundColor Cyan
$tunnel = Test-NetConnection -ComputerName 127.0.0.1 -Port 5435 -WarningAction SilentlyContinue -InformationLevel Quiet

if (-not $tunnel) {
    Write-Host ""
    Write-Host "  [ERROR] No hay nada escuchando en el puerto 5435." -ForegroundColor Red
    Write-Host "  Abre el tunel SSH primero y vuelve a ejecutar este script." -ForegroundColor Red
    Write-Host ""
    exit 1
}

Write-Host "  [OK] Puerto 5435 accesible. Continuando..." -ForegroundColor Green
Write-Host ""

# --- 3. Copiar .env.remote como backend/.env (toma prioridad) ---
Copy-Item $EnvRemote $EnvTemp -Force
Write-Host "  [OK] .env.remote copiado a backend\.env" -ForegroundColor Green

# --- 4. Arrancar el backend con limpieza garantizada al salir ---
Write-Host "  Arrancando backend (BD remota: appdb en 127.0.0.1:8085)..." -ForegroundColor Cyan
Write-Host "  Presiona Ctrl+C para detener y volver al modo local." -ForegroundColor Yellow
Write-Host ""

try {
    Set-Location $Backend
    go run cmd/main.go
} finally {
    Set-Location $Root
    if (Test-Path $EnvTemp) {
        Remove-Item $EnvTemp -Force
        Write-Host ""
        Write-Host "  [OK] backend\.env eliminado. Modo local restaurado." -ForegroundColor Green
        Write-Host ""
    }
}
