@echo off
setlocal
cd /d "%~dp0"

if not exist "gradlew.bat" (
  echo ERROR: gradlew.bat is missing.
  pause
  exit /b 1
)
if not exist "app\src\main\jniLibs\arm64-v8a\librkhcfs_go.so" (
  call BUILD_GO_ANDROID_ARM64.bat
  if errorlevel 1 exit /b 1
)
call gradlew.bat --stop >nul 2>&1
call gradlew.bat clean :app:assembleDebug --stacktrace
if errorlevel 1 (
  echo BUILD FAILED. Set Gradle JDK to JDK 17.
  pause
  exit /b 1
)
echo APK: app\build\outputs\apk\debug\app-debug.apk
pause
