@echo off
setlocal
cd /d "%~dp0"

set "PY="
where py >nul 2>nul && set "PY=py -3"
if not defined PY where python >nul 2>nul && set "PY=python"
if not defined PY (
  echo Python 3 is needed to run this app. Opening the download page...
  echo Tick "Add python.exe to PATH" during install, then run this file again.
  start "" https://www.python.org/downloads/windows/
  pause
  exit /b 1
)

echo Installing requirements (first run can take a minute)...
%PY% -m pip install --quiet --upgrade -r requirements.txt
if errorlevel 1 (
  echo Could not install the requirements.
  pause
  exit /b 1
)

for /f "delims=" %%i in ('%PY% -c "import sys, os; print(os.path.join(os.path.dirname(sys.executable), 'pythonw.exe'))"') do set "PYW=%%i"
if not exist "%PYW%" set "PYW=python"

start "" "%PYW%" "%~dp0airpods_battery.pyw"
echo AirPods Battery is running - look for it in the system tray.
timeout /t 3 >nul
