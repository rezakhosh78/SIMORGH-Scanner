@echo off
setlocal EnableExtensions
cd /d "%~dp0"

echo Build SIMORGH Scanner Go backend for Android arm64
where go >nul 2>nul
if errorlevel 1 (
  echo ERROR: Go is not installed or is not in PATH.
  pause
  exit /b 1
)
if not exist "go-backend\go.mod" (
  echo ERROR: go-backend\go.mod was not found.
  pause
  exit /b 1
)
if not exist "app\src\main\jniLibs\arm64-v8a" mkdir "app\src\main\jniLibs\arm64-v8a"
pushd go-backend
set "GOOS=android"
set "GOARCH=arm64"
set "CGO_ENABLED=0"
go build -trimpath -ldflags="-s -w" -o "..\app\src\main\jniLibs\arm64-v8a\librkhcfs_go.so" .
set "BUILD_EXIT=%ERRORLEVEL%"
popd
if not "%BUILD_EXIT%"=="0" (
  echo ERROR: Go backend build failed.
  pause
  exit /b %BUILD_EXIT%
)
echo Done: app\src\main\jniLibs\arm64-v8a\librkhcfs_go.so
pause
exit /b 0
