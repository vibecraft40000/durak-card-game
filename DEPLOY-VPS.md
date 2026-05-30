# Deploy on VPS

1. **Build frontend and copy files:**

```powershell
npm run build
New-Item -ItemType Directory -Force -Path docker\frontend-dist
Copy-Item -Path dist\* -Destination docker\frontend-dist -Recurse -Force
scp -r -o StrictHostKeyChecking=accept-new backend docker root@YOUR_SERVER_IP:/root/durakonline/
scp -o StrictHostKeyChecking=accept-new .env root@YOUR_SERVER_IP:/root/durakonline/
```

2. **Connect via SSH and run:**

```bash
cd /root/durakonline
sh docker/run-on-vps.sh
```

3. **Check:** Open `http://YOUR_SERVER_IP/` in browser.

4. **Telegram Mini App** requires HTTPS. Use a domain (e.g. DuckDNS) with Nginx + Certbot.
