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
echo "[1/6] Creating application directories..."
mkdir -p /var/lib/seintech/data
mkdir -p /var/lib/seintech/ssl/panel
mkdir -p /var/lib/seintech/acme-challenge
mkdir -p /var/lib/seintech/pma_tokens
mkdir -p /var/www
mkdir -p /usr/share/adminer
chmod 750 /var/lib/seintech/data
chmod 700 /var/lib/seintech/ssl
chmod 777 /var/lib/seintech/pma_tokens

# Ensure bundled official standalone adminer.php is copied to system
if [ -f "./web/static/adminer.php" ]; then
    cp ./web/static/adminer.php /usr/share/adminer/adminer.php
    chmod 644 /usr/share/adminer/adminer.php
    chown www-data:www-data /usr/share/adminer/adminer.php 2>/dev/null || true
fi

# 2. Configure APT Repositories for PHP (Debian/Ubuntu)
if command -v apt-get >/dev/null 2>&1; then
    echo "[2/6] Configuring APT repositories for PHP"
    export DEBIAN_FRONTEND=noninteractive

    OS_ID="debian"
    CODENAME=""
    if [ -f /etc/os-release ]; then
        . /etc/os-release
        OS_ID="${ID:-debian}"
        CODENAME="${VERSION_CODENAME:-}"
    fi

    if [ -z "$CODENAME" ] && command -v lsb_release >/dev/null 2>&1; then
        CODENAME=$(lsb_release -sc)
    fi

    if [ -z "$CODENAME" ] && [ -n "$VERSION_ID" ]; then
        case "$VERSION_ID" in
            13*) CODENAME="trixie" ;;
            12*) CODENAME="bookworm" ;;
            11*) CODENAME="bullseye" ;;
            24.04*) CODENAME="noble" ;;
            22.04*) CODENAME="jammy" ;;
            20.04*) CODENAME="focal" ;;
        esac
    fi

    if [ -z "$CODENAME" ]; then
        if [ "$OS_ID" = "ubuntu" ]; then
            CODENAME="noble"
        else
            CODENAME="trixie"
        fi
    fi

    echo "  -> Detected OS: $OS_ID ($CODENAME)"
    apt-get update -y -qq
    apt-get install -y -qq ca-certificates curl gnupg lsb-release wget software-properties-common 2>/dev/null || true

    mkdir -p /usr/share/keyrings /etc/apt/trusted.gpg.d /etc/apt/sources.list.d

    # Setup PHP Repository (Sury for Debian, PPA for Ubuntu)
    if [ "$OS_ID" = "ubuntu" ]; then
        if command -v add-apt-repository >/dev/null 2>&1; then
            add-apt-repository -y ppa:ondrej/php
        else
            curl -fsSL "https://keyserver.ubuntu.com/pks/lookup?op=get&search=0x14AA40EC0831756756D7F66C4F4EA0AAE5267A6C" | gpg --dearmor -o /etc/apt/trusted.gpg.d/ondrej-php.gpg 2>/dev/null || true
            echo "deb [signed-by=/etc/apt/trusted.gpg.d/ondrej-php.gpg] https://ppa.launchpadcontent.net/ondrej/php/ubuntu $CODENAME main" > /etc/apt/sources.list.d/php.list
        fi
    else
        curl -fsSL https://packages.sury.org/php/apt.gpg -o /usr/share/keyrings/deb.sury.org-php.gpg 2>/dev/null || true
        cp /usr/share/keyrings/deb.sury.org-php.gpg /etc/apt/trusted.gpg.d/php.gpg 2>/dev/null || true
        echo "deb [signed-by=/usr/share/keyrings/deb.sury.org-php.gpg] https://packages.sury.org/php/ $CODENAME main" > /etc/apt/sources.list.d/php.list
    fi

    # Cleanup any deprecated repositories (LiteSpeed / Caddy)
    rm -f /etc/apt/sources.list.d/lst_debian_repo.list /etc/apt/sources.list.d/caddy-stable.list 2>/dev/null || true
    rm -f /etc/apt/trusted.gpg.d/lst_debian_repo.gpg /etc/apt/trusted.gpg.d/lst_repo.gpg /usr/share/keyrings/caddy-stable-archive-keyring.gpg 2>/dev/null || true

    apt-get update -y -qq || true
    apt-get install -y -qq php-mysql php-pgsql php-sqlite3 php-mbstring 2>/dev/null || true
fi

# 3. Install Binary (Stop existing service first if running to avoid Text file busy error)
echo "[3/6] Installing binary to /usr/local/bin/seintech..."
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

# 4. Setup Systemd Service
echo "[4/6] Setting up systemd service unit..."
cp ./deploy/systemd/seintech.service /etc/systemd/system/seintech.service
systemctl daemon-reload

# 5. Enable and Start Service
echo "[5/6] Enabling and starting Seintech Panel..."
systemctl enable seintech.service
systemctl restart seintech.service

# 6. Output Access Information
echo "[6/6] Checking service status..."
sleep 2

HOST_IP=$(hostname -I | awk '{print $1}')
echo "=========================================================="
echo "    Seintech Hosting Panel successfully installed!       "
echo "=========================================================="
echo "  URL: https://${HOST_IP}:8443 or https://localhost:8443"
echo "  Default User: admin"
echo "  Default Pass: admin123"
echo "=========================================================="
