# Seintech Hosting Panel 🚀

**Seintech Hosting Panel** is an ultra-lightweight, modular, and resilient modern web hosting control panel written in **Go (Single Standalone Binary)**. It is specifically designed to run as a native `systemd` service on Linux distributions (Debian, Ubuntu) without requiring any external database server (such as MySQL or SQLite) for its internal state.

Official Repository: **[https://github.com/seintechid/panel](https://github.com/seintechid/panel)**

---

## 🌟 Key Architecture & Features

1. **Standalone Single Go Binary (`embed.FS`):**
   - All static assets (CSS, JS, branding, bundled Adminer core) and Go HTML templates are compiled directly into a single self-contained executable using Go's `embed.FS`.
   - Zero runtime dependencies: No Node.js, Python runtime, or interpreter is required on the host server to run the control panel.

2. **High-Integrity Flat-File JSON Storage Engine:**
   - Stores all system state and entities under `/var/lib/seintech/data/*.json`.
   - **Dual-Locking Mechanism:** Protected by both in-process concurrency control (`sync.RWMutex`) and OS-level file locking (`flock`) to prevent race conditions across concurrent processes.
   - **Atomic Write Strategy:** Writes mutations to temporary files first before executing an atomic `os.Rename`, guaranteeing data consistency even during unexpected power outages.

3. **Dual Web Server Management (Nginx Edge & Apache2 Backend):**
   - Unified `WebServer` abstraction managing **Nginx** (High-Performance Edge Reverse Proxy / Static Engine) and **Apache 2.4** (MPM Event & FastCGI).
   - Automated virtual host generator for reverse proxying, static websites, and PHP-FPM socket routing with HTTP/2 and modern SSL ciphers.

4. **Multi-PHP Version Engine (PHP 7.4 through PHP 8.5):**
   - Isolated PHP-FPM socket pools per version (`/run/php/phpX.Y-fpm.sock`).
   - Domain-level `php.ini` directive tuning (`upload_max_filesize`, `post_max_size`, `memory_limit`, `max_execution_time`).
   - Automated repository management with Ondřej Surý PPA / Debian packages repository.

5. **Unified Database Management via Adminer (MariaDB & PostgreSQL):**
   - Out-of-the-box support for both **MariaDB/MySQL** and **PostgreSQL**.
   - Complete database provisioning, user privilege management, and disk usage tracking.
   - Built-in **Adminer** integration replacing legacy phpMyAdmin and phpPgAdmin:
     - Bundled standalone, single-file official distribution.
     - Curated modern theme (`adminer.css`) with sleek typography, cards, and dark accents.
     - One-click **Single Sign-On (SSO)** auto-login directly into individual databases without retyping credentials.

6. **Automated SSL via Let's Encrypt ACME:**
   - Automated HTTP-01 challenge handling, self-signed TLS bootstrap certificates for initial setup, and background renewal workers running every 12 hours (renewing certificates 30 days prior to expiration).

7. **Secure Web File Manager & In-Browser Code Editor:**
   - Strict chroot path containment and path sanitization to eliminate Directory Traversal (`../`) and Symlink Breakout attacks.
   - Built-in ZipSlip exploit mitigation during archive extractions.
   - High-performance web code editor powered by monospace typography.

8. **Three-Tier Role-Based Access Control (RBAC) & Resource Quotas:**
   - **Root Admin**, **Reseller**, and **End-User** roles.
   - Cryptographically secure password hashing using `bcrypt`.
   - Resource quota enforcement: Storage limit (MB), Linux process memory (MB), max virtual hosts, and max database instances per account.
   - Strict tenant isolation: End-users are confined strictly to their dedicated site root and database instances.

---

## 📁 Project Directory Structure

```text
seintech-panel/
├── bin/
│   └── seintech                    # Precompiled Linux amd64 production binary (static CGO_ENABLED=0)
├── cmd/
│   └── seintech/
│       └── main.go                 # Application entrypoint, CLI flags, bootstrap admin & graceful shutdown
├── configs/
│   └── schema/                     # JSON schema definitions for flat-file database
│       ├── users.json
│       ├── domains.json
│       ├── databases.json
│       ├── quotas.json
│       └── settings.json
├── deploy/
│   ├── systemd/
│   │   └── seintech.service        # Systemd service unit definition with sandboxing policies
│   └── install.sh                  # Automated one-click production installer for Linux
├── internal/
│   ├── config/
│   │   └── config.go               # Environment configuration loader (Dev vs Production)
│   ├── core/
│   │   ├── auth/                   # Session authentication & bcrypt password hashing
│   │   ├── database/               # MariaDB & PostgreSQL provisioning engines & permission fixers
│   │   ├── filemanager/            # File operations, archive compression, ZipSlip guard & chroot
│   │   ├── i18n/                   # Multi-language translation engine (English & Indonesian)
│   │   ├── php/                    # Multi-version PHP 7.4-8.5 FPM pool manager & php.ini builder
│   │   ├── ssl/                    # ACME Let's Encrypt client, self-signed TLS & Nginx reverse proxy
│   │   ├── system/                 # Linux metrics, quota enforcement, service lifecycle & Adminer auto-config
│   │   └── webserver/              # WebServer interface abstraction (Nginx & Apache2 drivers)
│   ├── http/
│   │   ├── handler/                # HTTP controllers (Dashboard, Domains, Databases, Services, SSL, etc.)
│   │   ├── middleware/             # Security headers, Session authentication & RBAC role guards
│   │   └── router.go               # Chi HTTP router & reverse proxy to Adminer loopback
│   ├── model/                      # Domain models (User, Domain, Database, Quota, Settings, Service)
│   └── storage/                    # Atomic, dual-locking JSON flat-file storage engine
├── web/
│   ├── embed.go                    # Go embed.FS bundling all templates, assets & Adminer
│   ├── static/                     # Web assets (CSS styles, icons, branding images)
│   │   └── adminer.php             # Bundled standalone official Adminer distribution
│   └── templates/                  # Go HTML layout and page templates
├── go.mod                          # Go module dependencies
├── go.sum                          # Checksums for Go module dependencies
└── README.md                       # Project documentation
```

---

## 🚀 Production Installation Guide (Debian / Ubuntu)

### 1. Server Prerequisites
- Operating System: **Debian 11 (Bullseye), Debian 12 (Bookworm), Debian 13 (Trixie)** or **Ubuntu 20.04, 22.04, 24.04 LTS**.
- Access: **root** (or a user with full `sudo` privileges).
- Required open network ports:
  - `8443` : Panel Web Dashboard (HTTPS)
  - `80`   : HTTP Web Traffic / Let's Encrypt ACME Verification
  - `443`  : HTTPS Web Server Traffic
  - `22`   : SSH Remote Administration

---

### 2. Automated One-Click Installation (Recommended)

Connect to your server via SSH as root, then execute the following:

#### Step A: Install base packages
```bash
apt update -y && apt install -y git curl wget ca-certificates
```

#### Step B: Clone the official repository
```bash
git clone https://github.com/seintechid/panel.git /opt/seintech-panel
```

#### Step C: Run the installation script
```bash
cd /opt/seintech-panel
chmod +x deploy/install.sh
./deploy/install.sh
```

The automated installer will:
1. Create essential system directories (`/var/lib/seintech/data`, `/var/lib/seintech/ssl`, `/usr/share/adminer`, `/var/www`, etc.).
2. Configure official PHP repositories (Ondřej Surý PPA / Debian repository) and essential database connector extensions (`php-mysql`, `php-pgsql`, `php-sqlite3`, `php-mbstring`).
3. Deploy the precompiled standalone binary `/usr/local/bin/seintech`.
4. Register and configure the systemd service `/etc/systemd/system/seintech.service`.
5. Start and enable Seintech Panel to run automatically upon boot.

---

### 3. Firewall Configuration (UFW)

If you are using UFW (Uncomplicated Firewall) on Debian or Ubuntu, ensure the following ports are open:

```bash
ufw allow 8443/tcp comment "Seintech Panel HTTPS"
ufw allow 80/tcp comment "HTTP Web"
ufw allow 443/tcp comment "HTTPS Web"
ufw allow 22/tcp comment "SSH"
ufw reload
```

---

### 4. Accessing the Web Dashboard

Once installation completes, open your web browser:

- **URL:** `https://<YOUR_SERVER_IP>:8443`  
  *(Example: `https://203.0.113.10:8443`)*
- **SSL Notice:** Since the panel generates a secure self-signed TLS certificate during initial bootstrap, your browser may display a certificate warning. Click **Advanced** and select **Proceed to ... (unsafe)** to continue.
- **Default Credentials:**
  - **Username:** `admin`
  - **Password:** `admin123`

> [!IMPORTANT]
> Change the default admin password immediately after logging in for the first time via the **Users** or **Settings** menu.

---

### 5. Service Management (Systemd)

Manage the Seintech Panel process using standard `systemctl` commands:

```bash
# Check service status
systemctl status seintech

# Inspect real-time application logs
journalctl -u seintech -f

# Restart the panel service
systemctl restart seintech

# Stop the panel service
systemctl stop seintech

# Start the panel service
systemctl start seintech
```

---

### 6. Updating the Panel

To update Seintech Panel to the latest release from GitHub:

```bash
cd /opt/seintech-panel
git pull https://github.com/seintechid/panel.git main
chmod +x deploy/install.sh
./deploy/install.sh
```

The installer stops the running service, installs the updated binary, configures database tools, and restarts the daemon safely without data loss.

---

## 🛠️ Advanced Options & Development

### Compiling from Source (ARM64 / Custom Builds)
If your server runs an ARM64 processor (e.g., Oracle Cloud Ampere A1, Raspberry Pi, AWS Graviton) or you prefer building the binary yourself:

1. Install Go on Debian/Ubuntu:
   ```bash
   apt install -y golang-go
   ```
2. Build and install the binary:
   ```bash
   cd /opt/seintech-panel
   go build -ldflags="-s -w" -o /usr/local/bin/seintech ./cmd/seintech
   chmod 755 /usr/local/bin/seintech
   systemctl restart seintech
   ```

### Running in Local Development Mode
To develop or inspect the panel locally without altering system services:
```bash
go run ./cmd/seintech -dev -port 8443
```
In development mode, Seintech automatically creates a local sandbox directory (`./data`, `./vhosts`, `./ssl`) and generates testing self-signed certificates at `https://localhost:8443`.

---

## 📄 License
This project is proprietary software developed by SEINTECH Systems. All rights reserved.
