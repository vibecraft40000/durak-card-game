#!/usr/bin/env python3
"""Check sshd_config and enable TCP forwarding on VPS."""
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
    client = paramiko.SSHClient()
    client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    client.connect(VPS, username=USER, password=PASS, timeout=30)
    # Show relevant lines
    stdin, out, _ = client.exec_command("grep -n -E 'AllowTcpForwarding|GatewayPorts|Match' /etc/ssh/sshd_config")
    print("Current config:")
    print(out.read().decode())
    # Force set and ensure uncommented
    cmd = r"""
    cp /etc/ssh/sshd_config /etc/ssh/sshd_config.bak
    sed -i '/^#*AllowTcpForwarding/d' /etc/ssh/sshd_config
    sed -i '/^#*GatewayPorts/d' /etc/ssh/sshd_config
    echo "AllowTcpForwarding yes" >> /etc/ssh/sshd_config
    echo "GatewayPorts yes" >> /etc/ssh/sshd_config
    sshd -t 2>&1
    systemctl restart ssh 2>/dev/null || systemctl restart sshd
    sleep 2
    grep -E 'AllowTcpForwarding|GatewayPorts' /etc/ssh/sshd_config
    """
    stdin, out, err = client.exec_command(cmd)
    out.channel.recv_exit_status()
    print(out.read().decode())
    if err.read():
        print("stderr:", err.read().decode())
    client.close()

if __name__ == "__main__":
    main()
