#!/usr/bin/env python3
"""Enable TCP forwarding in sshd on VPS. Run once."""
import paramiko

VPS, USER, PASS = "72.56.74.7", "root", "azfzD1V+*gkevz"

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
