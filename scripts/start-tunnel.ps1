# SSH reverse tunnel: VPS:9080 -> localhost:5173
# Run after: docker up + npm run dev
# Keep this window OPEN. BotFather: https://72-56-74-7.sslip.io

Write-Host "VPS proxy tunnel. Keep open. Ctrl+C to stop." -ForegroundColor Yellow
ssh -o ServerAliveInterval=60 -R 9080:localhost:5173 root@72.56.74.7
