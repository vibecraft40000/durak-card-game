#!/usr/bin/env python3
"""Enable TCP forwarding in sshd on VPS. Run once."""
import os
import paramiko
import sys

VPS = os.environ.get("VPS_HOST", "YOUR_SERVER_IP")
USER = os.environ.get("VPS_USER", "root")
PASS = os.environ.get("VPS_PASSWORD", "").strip()

if not PASS:
    print("VPS_PASSWORD is required. The previously hardcoded VPS credential was removed from the repo and must be rotated out of band before reuse.")
    sys.exit(1)

def main():
    print("Connecting to VPS...")
    client = paramiko.SSHClient()
    client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    client.connect(VPS, username=USER, password=PASS, timeout=30)
    cmd = (
        "grep -q '^AllowTcpForwarding' /etc/ssh/sshd_config && "
        "sed -i 's/^AllowTcpForwarding.*/AllowTcpForwarding yes/' /etc/ssh/sshd_config || "
        "echo 'AllowTcpForwarding yes' >> /etc/ssh/sshd_config"
    )
    stdin, stdout, stderr = client.exec_command(cmd)
    stdout.channel.recv_exit_status()
    print("Updating sshd_config... done.")
    stdin, stdout, stderr = client.exec_command("systemctl restart ssh 2>/dev/null || systemctl restart sshd")
    stdout.channel.recv_exit_status()
    err = stderr.read().decode()
    if err and "Failed" in err:
        print("Warning:", err)
    else:
        print("sshd restarted. TCP forwarding enabled.")
    client.close()

if __name__ == "__main__":
    main()
