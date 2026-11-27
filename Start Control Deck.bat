@echo off
rem Double-click to start the Control Deck PC companion.
cd /d "%~dp0"
where py >nul 2>nul
if %errorlevel%==0 (
    py -3 companion\bridge.py
) else (
    python companion\bridge.py
)
pause
