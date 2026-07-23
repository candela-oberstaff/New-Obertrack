@echo off
setlocal enabledelayedexpansion
title Obertrack - Backend + App movil
chcp 65001 >nul

REM ============================================================
REM  run-mobile.bat
REM  1) Levanta el backend (docker compose)
REM  2) Espera a que responda /health
REM  3) Espera a que conectes el telefono por USB
REM  4) Lanza "flutter run" apuntando a la IP LAN de esta PC
REM
REM  Uso:
REM     run-mobile.bat            -> arranque normal
REM     run-mobile.bat build      -> reconstruye el backend (--build)
REM ============================================================

REM ---- Config (edita si cambia tu telefono o quieres fijar la IP) ----
set "DEVICE_ID=922bd9f2"
set "BACKEND_PORT=8080"
set "API_IP=192.168.100.88"

REM ---- Rebuild opcional del backend ----
set "BUILD_FLAG="
if /i "%~1"=="build" set "BUILD_FLAG=--build"

REM ---- Auto-detectar la IP LAN (192.168.x.x); si falla usa la de arriba ----
for /f "tokens=2 delims=:" %%a in ('ipconfig ^| findstr /c:"IPv4" ^| findstr /c:"192.168."') do (
    set "DETECTED=%%a"
)
if defined DETECTED (
    set "DETECTED=!DETECTED: =!"
    set "API_IP=!DETECTED!"
)

set "REPO_DIR=%~dp0"
cd /d "%REPO_DIR%"

echo ============================================
echo   Obertrack - arranque local
echo   API      : http://!API_IP!:%BACKEND_PORT%
echo   Telefono : %DEVICE_ID%
if defined BUILD_FLAG echo   Backend  : reconstruyendo (--build)
echo ============================================
echo.

REM ---- 1) Backend ----
echo [1/3] Levantando backend (docker compose up -d %BUILD_FLAG%)...
docker compose up -d %BUILD_FLAG%
if errorlevel 1 (
    echo.
    echo ERROR: fallo docker compose. Esta Docker Desktop corriendo?
    pause
    exit /b 1
)

REM ---- 2) Esperar /health ----
echo.
echo [2/3] Esperando a que el backend responda en /health ...
set /a _tries=0
:waitbackend
curl -s -o nul --max-time 3 http://localhost:%BACKEND_PORT%/health
if not errorlevel 1 goto backendok
set /a _tries+=1
if !_tries! geq 30 (
    echo ERROR: el backend no respondio tras 30 intentos.
    pause
    exit /b 1
)
timeout /t 2 >nul
goto waitbackend
:backendok
echo       Backend OK.

REM ---- 3) Esperar el telefono ----
echo.
echo [3/3] Conecta el telefono por USB (con depuracion USB activada)...
echo       Esperando el dispositivo %DEVICE_ID% ...
:waitphone
flutter devices 2>nul | findstr /c:"%DEVICE_ID%" >nul
if errorlevel 1 (
    timeout /t 3 >nul
    goto waitphone
)
echo       Telefono detectado.

REM ---- 4) Lanzar la app ----
echo.
echo Lanzando la app en el telefono...
echo (Si el login dice "No se pudo conectar", revisa que el telefono y la PC
echo  esten en la misma Wi-Fi y que el Firewall de Windows permita el puerto %BACKEND_PORT%.)
echo.
cd /d "%REPO_DIR%obertrack_mobile"
flutter run -d %DEVICE_ID% --dart-define=API_BASE_URL=http://!API_IP!:%BACKEND_PORT%

echo.
echo La app se cerro. Presiona una tecla para salir.
pause >nul
endlocal
