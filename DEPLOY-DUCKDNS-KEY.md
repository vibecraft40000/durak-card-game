# Deploy на durakonline.duckdns.org — только SSH по ключу

Сервер принимает **только publickey**. Нужно настроить SSH-ключ.

## 1. Где включить пароль или добавить ключ

Зайди в панель VPS (Hetzner, Selectel и т.п.) и открой **консоль/терминал** (VNC или встроенный терминал).

## 2. Вариант A: Включить вход по паролю

На сервере в консоли:

```bash
sed -i 's/^#*PasswordAuthentication.*/PasswordAuthentication yes/' /etc/ssh/sshd_config
sed -i 's/^#*PermitRootLogin.*/PermitRootLogin yes/' /etc/ssh/sshd_config
systemctl restart sshd
```

После этого можно использовать `deploy_duckdns.py` с паролем.

## 3. Вариант B: Добавить SSH-ключ

**На твоём ПК** — проверь, есть ли ключ:

```powershell
Get-Content $env:USERPROFILE\.ssh\id_rsa.pub
# или id_ed25519.pub
```

Если ключа нет — создай:

```powershell
ssh-keygen -t ed25519 -N "" -f $env:USERPROFILE\.ssh\id_ed25519
```

Скопируй содержимое `id_ed25519.pub`. **В консоли VPS**:

```bash
mkdir -p /root/.ssh
echo "ВСТАВЬ_СЮДА_СОДЕРЖИМОЕ_PUB_КЛЮЧА" >> /root/.ssh/authorized_keys
chmod 600 /root/.ssh/authorized_keys
```

Потом запусти:

```powershell
.\scripts\deploy-duckdns.ps1
```

(через deploy без пароля, если настроен ключ)

## 4. Ручной deploy (если ключ уже есть)

```powershell
# Сборка
npm run build
Copy-Item dist\* docker\frontend-dist -Recurse -Force

# Загрузка (без пароля, по ключу)
scp -r backend docker root@65.108.102.219:/root/durakonline/
scp .env root@65.108.102.219:/root/durakonline/

# На сервере
ssh root@65.108.102.219 "cd /root/durakonline && sh scripts/setup-duckdns-ssl.sh"
```
