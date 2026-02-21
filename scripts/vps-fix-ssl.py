#!/usr/bin/env python3
"""Fix 502: full deploy + run app on :8080, point host Nginx (SSL) at it. Run: python scripts/vps-fix-ssl.py"""
import os
import sys
import shutil
import tarfile
import tempfile
from pathlib import Path

try:
    import paramiko
except ImportError:
    print("pip install paramiko")
    sys.exit(1)

VPS_HOST = "72.56.74.7"
VPS_USER = "root"
VPS_PASSWORD = "azfzD1V+*gkevz"
REMOTE_PATH = "/root/durakonline"
PROJECT_ROOT = Path(__file__).resolve().parent.parent


def run(client, cmd, timeout=120):
    _, out, err = client.exec_command(cmd, timeout=timeout)
    code = out.channel.recv_exit_status()
    return code, out.read().decode("utf-8", errors="replace"), err.read().decode("utf-8", errors="replace")


def sftp_mkdir_p(sftp, path):
    parts = path.rstrip("/").split("/")
    for i in range(1, len(parts) + 1):
        sub = "/".join(parts[:i])
        if not sub:
            continue
        try:
            sftp.stat(sub)
        except FileNotFoundError:
            try:
                sftp.mkdir(sub)
            except OSError:
                pass


def create_tar_backend(exclude_dirs, dest_path: Path) -> int:
    """Create tar of backend excluding .gopath etc. Returns file count."""
    n = 0
    with tarfile.open(dest_path, "w:gz") as tf:
        backend = PROJECT_ROOT / "backend"
        for root, dirs, files in os.walk(backend):
            dirs[:] = [d for d in dirs if d not in exclude_dirs]
            root = Path(root)
            for f in files:
                p = root / f
                arc = p.relative_to(PROJECT_ROOT)
                tf.add(str(p), arcname=str(arc))
                n += 1
                if n % 100 == 0:
                    print(f"  packing {n}...", end="\r", flush=True)
    if n > 0:
        print(f"  packed {n} files.    ", flush=True)
    return n


def main():
    print("Connecting to VPS...", flush=True)
    client = paramiko.SSHClient()
    client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    client.connect(VPS_HOST, username=VPS_USER, password=VPS_PASSWORD, timeout=30)
    sftp = paramiko.SFTPClient.from_transport(client.get_transport())

    sftp_mkdir_p(sftp, REMOTE_PATH)
    sftp_mkdir_p(sftp, REMOTE_PATH + "/docker")

    # 1) Build frontend and prepare frontend-dist
    print("Building frontend...", flush=True)
    r = os.system("npm run build")
    if r != 0:
        print("npm run build failed")
        return 1
    frontend_dist = PROJECT_ROOT / "docker" / "frontend-dist"
    frontend_dist.mkdir(parents=True, exist_ok=True)
    dist = PROJECT_ROOT / "dist"
    for f in dist.iterdir():
        dst = frontend_dist / f.name
        if f.is_file():
            shutil.copy2(str(f), str(dst))
        else:
            shutil.copytree(str(f), str(dst), dirs_exist_ok=True)

    # 2) Create tars and upload (1 file each = much faster than 5900 files)
    with tempfile.TemporaryDirectory() as td:
        backend_tar = Path(td) / "backend.tar.gz"
        docker_tar = Path(td) / "docker.tar.gz"
        print("Packing backend (excluding .gopath)...", flush=True)
        create_tar_backend([".gopath", "node_modules", "__pycache__", ".git"], backend_tar)
        print("Packing docker...", flush=True)
        docker_dir = PROJECT_ROOT / "docker"
        with tarfile.open(docker_tar, "w:gz") as tf:
            for root, dirs, files in os.walk(docker_dir):
                dirs[:] = [d for d in dirs if d != ".git"]
                for f in files:
                    p = Path(root) / f
                    arc = Path("docker") / p.relative_to(docker_dir)
                    tf.add(str(p), arcname=str(arc))
        print("Uploading backend.tar.gz...", flush=True)
        sftp.put(str(backend_tar), "/tmp/durak-backend.tar.gz")
        print("Uploading docker.tar.gz...", flush=True)
        sftp.put(str(docker_tar), "/tmp/durak-docker.tar.gz")
    sftp.put(str(PROJECT_ROOT / "docker" / "nginx-host-sslip.conf"), "/tmp/durak-nginx-sslip.conf")
    if (PROJECT_ROOT / ".env").exists():
        sftp.put(str(PROJECT_ROOT / ".env"), "/tmp/durak.env")
    sftp.close()

    # Extract on server
    print("Extracting on VPS...", flush=True)
    run(client, f"mkdir -p {REMOTE_PATH} && cd {REMOTE_PATH} && tar -xzf /tmp/durak-backend.tar.gz 2>/dev/null || true")
    run(client, f"cd {REMOTE_PATH} && tar -xzf /tmp/durak-docker.tar.gz 2>/dev/null || true")
    run(client, "cp /tmp/durak-nginx-sslip.conf /tmp/ 2>/dev/null; true")
    run(client, f"[ -f /tmp/durak.env ] && cp /tmp/durak.env {REMOTE_PATH}/.env; true")

    # 3) Start app (use docker-compose v1 if "docker compose" not available)
    print("Starting Docker (app on port 8080)...", flush=True)
    cmd = f"cd {REMOTE_PATH} && (docker compose -f docker/docker-compose.vps.yml -f docker/docker-compose.override.vps.yml up -d --build 2>/dev/null || docker-compose -f docker/docker-compose.vps.yml -f docker/docker-compose.override.vps.yml up -d --build)"
    code, out, err = run(client, cmd, timeout=300)
    print(out[-2500:] if len(out) > 2500 else out)
    if err:
        print("stderr:", err[-1000:])
    if code != 0:
        print("Docker failed. Listing containers...")
        _, o, _ = run(client, "docker ps -a")
        print(o)
        client.close()
        return 1

    # 3) Enable our nginx site and reload host nginx
    print("Configuring host Nginx...", flush=True)
    run(client, "sudo rm -f /etc/nginx/sites-enabled/*72*56*74* 2>/dev/null; "
                "sudo rm -f /etc/nginx/sites-enabled/*sslip* 2>/dev/null; true")
    code, o, e = run(client, "sudo cp /tmp/durak-nginx-sslip.conf /etc/nginx/sites-available/durak-72-56-74-7.sslip.io && "
                         "sudo ln -sf /etc/nginx/sites-available/durak-72-56-74-7.sslip.io /etc/nginx/sites-enabled/durak-72-56-74-7.sslip.io && "
                         "sudo nginx -t 2>&1")
    print(o or e)
    if code != 0:
        print("Nginx -t failed. Trying without SSL block (HTTP only)...")
        # Fallback: create HTTP-only config if SSL paths missing
        run(client, "sudo sed -n '1,/^server {/p' /tmp/durak-nginx-sslip.conf | head -n -1 > /tmp/durak-http.conf; "
            "echo '}' >> /tmp/durak-http.conf; sudo cp /tmp/durak-http.conf /etc/nginx/sites-available/durak-72-56-74-7.sslip.io")
        run(client, "sudo nginx -t 2>&1")
    run(client, "sudo systemctl reload nginx 2>&1")
    print("Nginx reloaded.", flush=True)

    print("Done. Open https://72-56-74-7.sslip.io/", flush=True)
    client.close()
    return 0


if __name__ == "__main__":
    sys.exit(main())
