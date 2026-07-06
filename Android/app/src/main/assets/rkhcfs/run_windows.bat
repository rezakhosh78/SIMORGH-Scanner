@echo off
cd /d "%~dp0"
py -m pip install -r requirements.txt
py RKh-CFS-v0.2.0.py
pause
