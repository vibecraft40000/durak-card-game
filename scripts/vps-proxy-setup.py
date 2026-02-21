#!/usr/bin/env python3
"""Setup VPS as proxy - upload nginx config. Run once."""
import sys
from pathlib import Path
import paramiko

VPS, USER, PASS = "72.56.74.7", "root", "azfzD1V+*gkevz"
CONF = Path(__file__).parent.parent / "docker" / "nginx-vps-proxy.conf"

def main():
    if not CONF.exists():
        print(f"Missing {CONF}")
        return 1
    print("Connecting to VPS...")
    client = paramiko.SSHClient()
    client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    client.connect(VPS, username=USER, password=PASS, timeout=30)
    sftp = paramiko.SFTPClient.from_transport(client.get_transport())
    sftp.put(str(CONF), "/tmp/durak-proxy.conf")
    sftp.close()
    _, out, _ = client.exec_command(
        "sudo cp /tmp/durak-proxy.conf /etc/nginx/sites-available/durak-proxy && "
        "sudo ln -sf /etc/nginx/sites-available/durak-proxy /etc/nginx/sites-enabled/ && "
        "sudo nginx -t && sudo systemctl reload nginx"
    )
    print(out.read().decode())
    client.close()
    print("Done. Run start-tunnel.ps1 and keep it open.")
    return 0

if __name__ == "__main__":
    sys.exit(main())
