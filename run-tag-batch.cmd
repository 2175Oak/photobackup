@echo off
cd /d "%~dp0"
if not exist "%~dp0build\bin\photo-backup-clean.exe" (
  echo Build not found: build\bin\photo-backup-clean.exe
  pause
  exit /b 1
)
start "" "%~dp0build\bin\photo-backup-clean.exe"
