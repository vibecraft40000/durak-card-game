@echo off
cd /d "%~dp0"
echo Installing dependencies...
python -m pip install -q -r requirements.txt
echo Starting bot...
python main.py
pause
