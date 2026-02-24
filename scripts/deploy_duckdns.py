#!/usr/bin/env python3
"""Deploy to durakonline.duckdns.org via SSH/SCP."""
import os
import subprocess
import sys
from pathlib import Path

PROJECT_ROOT = Path(__file__).resolve().parent.parent
HOST = "72.56.74.7"
USER = "root"
REMOTE_PATH = "/root/durakonline"


def main():
    pw = os.environ.get("DEPLOY_PW", "")
    if not pw and len(sys.argv) > 1:
        pw = sys.argv[1]
    if not pw:
        pw_file = PROJECT_ROOT / ".deploy-pw"
        if pw_file.exists():
            pw = pw_file.read_text(encoding="utf-8").strip()
    if not pw:
        import getpass
        pw = getpass.getpass("Password (root@72.56.74.7): ")

    try:
        import paramiko
        from scp import SCPClient
    except ImportError:
        print("Install: pip install paramiko scp")
        sys.exit(1)

    os.chdir(PROJECT_ROOT)
    print("1. Building frontend...")
    subprocess.run("npm run build", shell=True, check=True, cwd=PROJECT_ROOT)
    (PROJECT_ROOT / "docker" / "frontend-dist").mkdir(parents=True, exist_ok=True)
    import shutil
    shutil.copytree(PROJECT_ROOT / "dist", PROJECT_ROOT / "docker" / "frontend-dist", dirs_exist_ok=True)

    print("2. Connecting...")
    ssh = paramiko.SSHClient()
    ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    ssh.connect(HOST, username=USER, password=pw)

    def put_exclude(local_dir, remote_base, exclude_dirs=(".gopath", ".gocache", "node_modules", "__pycache__")):
        sftp = ssh.open_sftp()
        local = Path(local_dir)
        for root, dirs, files in os.walk(local):
            dirs[:] = [d for d in dirs if d not in exclude_dirs]
            rel = Path(root).relative_to(local)
            remote_dir = f"{remote_base}/{rel}".replace("\\", "/")
            try:
                sftp.stat(remote_dir)
            except OSError:
                sftp.mkdir(remote_dir)
            for f in files:
                sftp.put(str(Path(root) / f), f"{remote_dir}/{f}")
        sftp.close()

    print("3. Uploading backend (excluding .gopath)...")
    put_exclude(PROJECT_ROOT / "backend", f"{REMOTE_PATH}/backend")
    print("4. Uploading docker...")
    with SCPClient(ssh.get_transport()) as scp:
        scp.put(str(PROJECT_ROOT / "docker"), REMOTE_PATH, recursive=True)
        scp.put(str(PROJECT_ROOT / ".env"), f"{REMOTE_PATH}/.env")

    print("5. Applying migrations...")
    _, stdout, stderr = ssh.exec_command(
        f"cd {REMOTE_PATH} && docker compose -f docker/docker-compose.caddy.yml run --rm migrate"
    )
    out, err = stdout.read().decode(), stderr.read().decode()
    print(out)
    if err and "no change" not in err.lower() and "error" in err.lower():
        print(err, file=sys.stderr)

    print("6. Rebuilding and restarting backend...")
    _, stdout, stderr = ssh.exec_command(
        f"cd {REMOTE_PATH} && "
        "docker compose -f docker/docker-compose.caddy.yml up -d --build --force-recreate api"
    )
    print(stdout.read().decode())
    err = stderr.read().decode()
    if err:
        print(err, file=sys.stderr)

    print("7. Restarting Caddy (frontend)...")
    _, stdout, stderr = ssh.exec_command(
        f"cd {REMOTE_PATH} && docker compose -f docker/docker-compose.caddy.yml restart caddy"
    )
    print(stdout.read().decode())
    err = stderr.read().decode()
    if err:
        print(err, file=sys.stderr)
    ssh.close()
    print("\nDone. https://durakonline.duckdns.org")


if __name__ == "__main__":
    main()
