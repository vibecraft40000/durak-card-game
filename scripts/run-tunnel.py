#!/usr/bin/env python3
"""SSH reverse tunnel via paramiko: VPS:9080 -> localhost:5173"""
import socket
import select
import threading
import paramiko

import sys
VPS, USER, PASS = "72.56.74.7", "root", "azfzD1V+*gkevz"
REMOTE_PORT = 9080
LOCAL_PORT = int(sys.argv[1]) if len(sys.argv) > 1 else 5173

def main():
    print("Connecting to VPS...")
    client = paramiko.SSHClient()
    client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    client.connect(VPS, username=USER, password=PASS, timeout=30)
    transport = client.get_transport()
    transport.request_port_forward("127.0.0.1", REMOTE_PORT)
    print(f"Tunnel OK: VPS:{REMOTE_PORT} -> localhost:{LOCAL_PORT}")
    print("App: https://72-56-74-7.sslip.io")
    try:
        while transport.is_active():
            chan = transport.accept(10)
            if chan:
                t = threading.Thread(target=handler, args=(chan,))
                t.daemon = True
                t.start()
    except (KeyboardInterrupt, EOFError):
        pass
    transport.cancel_port_forward("127.0.0.1", REMOTE_PORT)
    client.close()

def handler(chan):
    sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    try:
        sock.connect(("127.0.0.1", LOCAL_PORT))
    except Exception:
        chan.close()
        return
    def fwd(a, b):
        while True:
            r, _, _ = select.select([a, b], [], [])
            if a in r:
                d = a.recv(65536)
                if not d:
                    return
                b.sendall(d)
            if b in r:
                d = b.recv(65536)
                if not d:
                    return
                a.sendall(d)
    t1 = threading.Thread(target=fwd, args=(chan, sock))
    t2 = threading.Thread(target=fwd, args=(sock, chan))
    t1.daemon = t2.daemon = True
    t1.start()
    t2.start()
    t1.join()
    t2.join()
    chan.close()
    sock.close()

if __name__ == "__main__":
    main()
