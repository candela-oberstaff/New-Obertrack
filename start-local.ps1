# ============================================================
# start-local.ps1 -- Arranca el backend apuntando a la BD local (Docker)
# Uso: .\start-local.ps1  (desde la raiz del proyecto)
# ============================================================

$ErrorActionPreference = "Stop"
$Root    = $PSScriptRoot
$Backend = Join-Path $Root "backend"
$EnvTemp = Join-Path $Backend ".env"

# --- 1. Limpiar backend/.env si quedo del modo remoto ---
if (Test-Path $EnvTemp) {
    Remove-Item $EnvTemp -Force
    Write-Host ""
    Write-Host "  [OK] Se elimino backend\.env del modo remoto anterior." -ForegroundColor Green
}

# --- 2. Informacion del modo ---
Write-Host ""
Write-Host "========================================================" -ForegroundColor Cyan
Write-Host "  MODO LOCAL -- BD Docker (obertrack en localhost:5432)" -ForegroundColor Cyan
Write-Host "========================================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "  Usando: New-Obertrack\.env" -ForegroundColor White
Write-Host "  BD:     obertrack @ localhost:5432 (Docker)" -ForegroundColor White
Write-Host ""
Write-Host "  Asegurate de que el contenedor Docker este corriendo." -ForegroundColor Yellow
Write-Host "  Si no esta activo: docker compose up -d" -ForegroundColor Yellow
Write-Host ""
Write-Host "  Presiona Ctrl+C para detener el backend." -ForegroundColor Yellow
Write-Host ""

# --- 3. Arrancar el backend ---
try {
    Set-Location $Backend
    go run cmd/main.go
} finally {
    Set-Location $Root
}
