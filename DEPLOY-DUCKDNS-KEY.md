# Deploy via SSH key

Server accepts **publickey** only. Set up SSH key before deploying.

## 1. Generate SSH key (if you don't have one)

```powershell
ssh-keygen -t ed25519 -N "" -f $env:USERPROFILE\.ssh\id_ed25519
```

Copy the public key and add it to your VPS provider's control panel.

## 2. Deploy

```powershell
.\scripts\deploy-vps.ps1
```
