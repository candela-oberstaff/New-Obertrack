# ============================================================
# start-remote2.ps1 -- Arranca el backend apuntando a la BD remota 2
# Uso: .\start-remote2.ps1  (desde la raiz del proyecto)
# ============================================================

$ErrorActionPreference = "Stop"
$Root      = $PSScriptRoot
$Backend   = Join-Path $Root "backend"
$EnvRemote = Join-Path $Root ".env.remote2"
$EnvTemp   = Join-Path $Backend ".env"

# --- 0. Verificar que .env.remote2 existe ---
if (-not (Test-Path $EnvRemote)) {
    Write-Host ""
    Write-Host "  [ERROR] No se encontro el archivo .env.remote2" -ForegroundColor Red
    Write-Host ""
    exit 1
}

# --- 1. Recordatorio del tunel SSH ---
Write-Host ""
Write-Host "========================================================" -ForegroundColor Cyan
Write-Host "  MODO REMOTO 2 -- BD en servidor 194.163.170.245" -ForegroundColor Cyan
Write-Host "========================================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "  ANTES de continuar, asegurate de tener el tunel SSH" -ForegroundColor Yellow
Write-Host "  abierto en otra terminal (apuntando a PostgreSQL):" -ForegroundColor Yellow
Write-Host ""
Write-Host "    ssh -L 5436:IP_DEL_CONTENEDOR:5432 root@194.163.170.245" -ForegroundColor White
Write-Host "    Password: 20759364" -ForegroundColor White
Write-Host ""

# --- 2. Verificar que el puerto 5436 responde ---
Write-Host "  Verificando tunel SSH en localhost:5436..." -ForegroundColor Cyan
$tunnel = Test-NetConnection -ComputerName 127.0.0.1 -Port 5436 -WarningAction SilentlyContinue -InformationLevel Quiet

if (-not $tunnel) {
    Write-Host ""
    Write-Host "  [ERROR] No hay nada escuchando en el puerto 5436." -ForegroundColor Red
    Write-Host "  Abre el tunel SSH primero y vuelve a ejecutar este script." -ForegroundColor Red
    Write-Host ""
    exit 1
}

Write-Host "  [OK] Puerto 5436 accesible. Continuando..." -ForegroundColor Green
Write-Host ""

# --- 3. Copiar .env.remote2 como backend/.env ---
Copy-Item $EnvRemote $EnvTemp -Force
Write-Host "  [OK] .env.remote2 copiado a backend\.env" -ForegroundColor Green

# --- 4. Arrancar el backend ---
Write-Host "  Arrancando backend (BD remota: 127.0.0.1:5436)..." -ForegroundColor Cyan
Write-Host "  Presiona Ctrl+C para detener." -ForegroundColor Yellow
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
