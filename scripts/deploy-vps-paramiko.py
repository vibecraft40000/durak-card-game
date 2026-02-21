# Deploy to VPS via SSH (password). Run: python scripts/deploy-vps-paramiko.py
# Requires: pip install paramiko

import os
import sys
import stat
from pathlib import Path

try:
    import paramiko
except ImportError:
    print("Install paramiko: pip install paramiko")
    sys.exit(1)

VPS_HOST = "72.56.74.7"
VPS_USER = "root"
VPS_PASSWORD = "azfzD1V+*gkevz"
REMOTE_PATH = "/root/durakonline"
PROJECT_ROOT = Path(__file__).resolve().parent.parent


def ignore_dir(name):
    return name in (".gopath", "node_modules", "__pycache__", ".git", "dist")


def sftp_mkdir_p(sftp, path: str):
    parts = path.rstrip("/").split("/")
    for i in range(1, len(parts) + 1):
        sub = "/".join(parts[:i])
        if not sub:
            continue
        try:
            sftp.stat(sub)
        except FileNotFoundError:
            sftp.mkdir(sub)


def upload_dir(sftp, local: Path, remote_path: str):
    for root, dirs, files in os.walk(local):
        dirs[:] = [d for d in dirs if not ignore_dir(d)]
        root = Path(root)
        rel = root.relative_to(local)
        rdir = f"{remote_path}/{rel.as_posix()}" if rel.parts else remote_path
        sftp_mkdir_p(sftp, rdir)
        for f in files:
            lf = root / f
            rf = f"{rdir}/{f}"
            try:
                sftp.put(str(lf), rf)
            except Exception as e:
                print(f"  put {lf} -> {rf}: {e}")


def run_ssh(cmd: str, client: paramiko.SSHClient) -> tuple[int, str, str]:
    _, stdout, stderr = client.exec_command(cmd, timeout=300)
    out = stdout.read().decode("utf-8", errors="replace")
    err = stderr.read().decode("utf-8", errors="replace")
    code = stdout.channel.recv_exit_status()
    return code, out, err


def main():
    os.chdir(PROJECT_ROOT)
    print("Building frontend...")
    r = os.system("npm run build")
    if r != 0:
        print("npm run build failed")
        sys.exit(1)
    import shutil
    frontend_dist = PROJECT_ROOT / "docker" / "frontend-dist"
    frontend_dist.mkdir(parents=True, exist_ok=True)
    dist = PROJECT_ROOT / "dist"
    for f in dist.iterdir():
        dst = frontend_dist / f.name
        if f.is_file():
            shutil.copy2(str(f), str(dst))
        else:
            shutil.copytree(str(f), str(dst), dirs_exist_ok=True)
    print("Connecting to VPS...", flush=True)
    client = paramiko.SSHClient()
    client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    client.connect(VPS_HOST, username=VPS_USER, password=VPS_PASSWORD, timeout=30)
    print("Uploading backend...", flush=True)
    transport = client.get_transport()
    sftp = paramiko.SFTPClient.from_transport(transport)
    try:
        sftp.mkdir(REMOTE_PATH)
    except OSError:
        pass
    upload_dir(sftp, PROJECT_ROOT / "backend", f"{REMOTE_PATH}/backend")
    print("Uploading docker...", flush=True)
    upload_dir(sftp, PROJECT_ROOT / "docker", f"{REMOTE_PATH}/docker")
    env_file = PROJECT_ROOT / ".env"
    if env_file.exists():
        sftp.put(str(env_file), f"{REMOTE_PATH}/.env")
        print("Uploaded .env")
    sftp.close()
    print("Running docker compose on VPS...", flush=True)
    cmd = f"cd {REMOTE_PATH} && docker compose -f docker/docker-compose.vps.yml up -d --build"
    code, out, err = run_ssh(cmd, client)
    print(out)
    if err:
        print("stderr:", err)
    client.close()
    if code != 0:
        print("Docker compose exit code:", code)
        sys.exit(1)
    print("Done. App: https://72-56-74-7.sslip.io/", flush=True)
    return 0


if __name__ == "__main__":
    sys.exit(main())
