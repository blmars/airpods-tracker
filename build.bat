@echo off
rem Builds AirPodsBattery.exe (needs Go 1.22+ from https://go.dev/dl/).
cd /d "%~dp0"
go build -trimpath -ldflags "-H windowsgui -s -w" -o AirPodsBattery.exe .
if errorlevel 1 (pause & exit /b 1)
echo Built AirPodsBattery.exe
