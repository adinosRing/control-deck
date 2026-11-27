@echo off
rem Double-click to set up Control Deck on this PC.
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0setup.ps1"
if errorlevel 1 pause
