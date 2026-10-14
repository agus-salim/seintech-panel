#!/usr/bin/env bash
# ==============================================================================
# Seintech Hosting Panel - Production Installer for Linux (Ubuntu/Debian/RHEL)
# ==============================================================================
set -e

echo "=========================================================="
echo "    Installing Seintech Hosting Panel (Single Binary)     "
echo "=========================================================="

if [ "$EUID" -ne 0 ]; then
  echo "Error: Please run this installer as root."
  exit 1
fi

# 1. Create System Directories
echo "[1/5] Creating application directories..."
mkdir -p /var/lib/seintech/data
mkdir -p /var/lib/seintech/ssl/panel
mkdir -p /var/lib/seintech/acme-challenge
mkdir -p /var/www
chmod 750 /var/lib/seintech/data
chmod 700 /var/lib/seintech/ssl

# 2. Install Binary (Stop existing service first if running to avoid Text file busy error)
echo "[2/5] Installing binary to /usr/local/bin/seintech..."
systemctl stop seintech.service 2>/dev/null || true

if [ -f "./bin/seintech" ]; then
    install -m 755 ./bin/seintech /usr/local/bin/seintech
elif [ -f "./seintech" ]; then
    install -m 755 ./seintech /usr/local/bin/seintech
else
    echo "Compiling binary on host..."
    go build -ldflags="-s -w" -o /usr/local/bin/seintech ./cmd/seintech
    chmod 755 /usr/local/bin/seintech
fi

# 3. Setup Systemd Service
echo "[3/5] Setting up systemd service unit..."
cp ./deploy/systemd/seintech.service /etc/systemd/system/seintech.service
systemctl daemon-reload

# 4. Enable and Start Service
echo "[4/5] Enabling and starting Seintech Panel..."
systemctl enable seintech.service
systemctl restart seintech.service

# 5. Output Access Information
echo "[5/5] Checking service status..."
sleep 2

HOST_IP=$(hostname -I | awk '{print $1}')
echo "=========================================================="
echo "    Seintech Hosting Panel successfully installed!       "
echo "=========================================================="
echo "  URL: https://${HOST_IP}:8443 or https://localhost:8443"
echo "  Default User: admin"
echo "  Default Pass: admin123"
echo "=========================================================="
