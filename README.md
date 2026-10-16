# Seintech Hosting Panel 🚀

**Seintech Hosting Panel** adalah web hosting control panel modern berbasis **single binary Go** yang ultra-ringan, modular, dan tangguh. Dirancang khusus untuk berjalan sebagai `systemd` service pada sistem operasi Linux (Ubuntu, Debian) tanpa memerlukan database eksternal untuk konfigurasi internalnya.

Repositori Resmi: **[https://github.com/seintechid/panel](https://github.com/seintechid/panel)**

---

## 🌟 Arsitektur & Keunggulan Utama

1. **Standalone Single Binary Go (`embed.FS`):**
   - Seluruh static assets (CSS, JS, gambar logo) dan Go HTML templates dibundle langsung ke dalam satu binary menggunakan fitur Go `embed.FS`.
   - Seluruh static assets (CSS, JS, gambar logo) dan Go HTML templates dibundle langsung ke dalam satu binary menggunakan fitur Go `embed.FS`.
   - Tidak memerlukan dependensi Node.js, runtime Python, atau interpreter di host server.
   
2. **Penyimpanan Flat-File JSON Berintegritas Tinggi:**
   - Menyimpan seluruh konfigurasi di `/var/lib/seintech/data/*.json`.
   - **Dual-Locking:** Dilindungi oleh `sync.RWMutex` (in-process concurrency safety) dan OS-level `flock` (advisory file locking) untuk mencegah race condition antar-proses.
   - **Atomic Write:** Menggunakan strategi penulisan ke temporary file kemudian `os.Rename` untuk mencegah kerusakan data jika terjadi pemadaman mendadak.

3. **Multi-Webserver Management (Nginx & Apache2):**
   - Abstraksi interface `WebServer` yang mengendalikan **Nginx** (High-Performance Engine / Reverse Proxy) dan **Apache2** (MPM Event & PHP-FPM).
3. **Multi-Webserver Management (Nginx & Apache2):**
   - Abstraksi interface `WebServer` yang mengendalikan **Nginx** (High-Performance Engine / Reverse Proxy) dan **Apache2** (MPM Event & PHP-FPM).
   - Dilengkapi template generator vhost untuk reverse proxy, static site, dan PHP-FPM socket router.

4. **Multi-PHP Version Engine (7.4 s/d 8.5):**
   - Mendukung eksekusi pool PHP-FPM individual (`/run/php/phpX.Y-fpm.sock`).
   - Pengaturan `php.ini` dasar per-domain (`upload_max_filesize`, `memory_limit`, `max_execution_time`).

5. **Multi-Database Engine:**
   - Mendukung **MariaDB/MySQL** dan **PostgreSQL**.
   - Pengelolaan database, user privilege, monitoring ukuran database, dan integrasi Single Sign-On (SSO) instan ke **phpMyAdmin** dan **phpPgAdmin**.
   - Pengelolaan database, user privilege, monitoring ukuran database, dan integrasi Single Sign-On (SSO) instan ke **phpMyAdmin** dan **phpPgAdmin**.

6. **Automasi SSL / Let's Encrypt ACME:**
   - HTTP-01 challenge otomatis, self-signed TLS bootstrap, dan cron worker auto-renewal tiap 12 jam (memperbarui sertifikat 30 hari sebelum kedaluwarsa).

7. **Web File Manager & Code Editor Aman:**
   - Chroot path containment & sanitizer untuk mencegah Directory Traversal (`../`) dan Symlink Breakout attacks.
   - Dilengkapi proteksi ZipSlip saat mengekstrak arsip.
   - Built-in web editor dengan font monospace **JetBrains Mono**.

8. **RBAC 3 Tingkat & Quota Resource:**
   - Root Admin, Reseller, dan End-User.
   - Password terenkripsi dengan algoritma `bcrypt`.
   - Pembatasan Kuota Penyimpanan / Storage (MB), RAM proses Linux (MB), jumlah virtualhost, dan database per user.
   - Isolasi ketat: End-User terkunci pada 1 situs, direktori miliknya, dan 1 database miliknya.
   - Pembatasan Kuota Penyimpanan / Storage (MB), RAM proses Linux (MB), jumlah virtualhost, dan database per user.
   - Isolasi ketat: End-User terkunci pada 1 situs, direktori miliknya, dan 1 database miliknya.

---

## 📁 Struktur Direktori Proyek

```text
seintech-panel/
├── bin/
│   ├── seintech                    # Precompiled Linux standalone binary
│   └── seintech-linux-amd64        # Salinan binary deployment Linux x86_64
├── bin/
│   ├── seintech                    # Precompiled Linux standalone binary
│   └── seintech-linux-amd64        # Salinan binary deployment Linux x86_64
├── cmd/
│   └── seintech/
│       └── main.go                 # Entrypoint aplikasi, CLI flags, bootstrap admin & graceful shutdown
├── configs/
│   └── schema/                     # Contoh skema flat-file JSON
│       ├── users.json
│       ├── domains.json
│       ├── databases.json
│       ├── databases.json
│       ├── quotas.json
│       └── settings.json
├── deploy/
│   ├── systemd/
│   │   └── seintech.service        # Systemd service unit dengan sandboxing
│   └── install.sh                  # Skrip instalasi otomatis Linux
├── internal/
│   ├── config/
│   │   └── config.go               # Loader konfigurasi lingkungan (dev vs prod)
│   ├── core/
│   │   ├── auth/
│   │   │   └── auth.go             # Manajemen sesi & enkripsi bcrypt
│   │   ├── database/
│   │   │   └── database.go         # Driver provisioning MariaDB & PostgreSQL
│   │   ├── filemanager/
│   │   │   ├── filemanager.go      # Operasi file, kompresi, ZipSlip guard
│   │   │   └── security.go         # Chroot jail & sanitasi symlink breakout
│   │   ├── php/
│   │   │   └── php.go              # Manajemen engine PHP 7.4-8.5 & FPM pools
│   │   ├── ssl/
│   │   │   └── ssl.go              # ACME client, self-signed generator & auto-renewal
│   │   ├── system/
│   │   │   ├── metrics.go          # Pengambilan metrik CPU, RAM, Disk, LoadAvg
│   │   │   ├── metrics_linux.go    # Pengambilan metrik spesifik kernel Linux
│   │   │   ├── quota.go            # Kalkulasi penggunaan Storage (MB) & RAM proses
│   │   │   ├── service.go          # Wrapper aman systemctl dengan whitelist
│   │   │   ├── user.go             # Linux user provisioning & home directory isolation
│   │   │   └── jobs.go             # Background package installation & progress tracker
│   │   │   ├── metrics_linux.go    # Pengambilan metrik spesifik kernel Linux
│   │   │   ├── quota.go            # Kalkulasi penggunaan Storage (MB) & RAM proses
│   │   │   ├── service.go          # Wrapper aman systemctl dengan whitelist
│   │   │   ├── user.go             # Linux user provisioning & home directory isolation
│   │   │   └── jobs.go             # Background package installation & progress tracker
│   │   └── webserver/
│   │       ├── webserver.go        # Interface WebServer & Registry
│   │       ├── nginx.go            # Implementasi Nginx vhost & reload
│   │       └── apache.go           # Implementasi Apache2 vhost & proxy_fcgi
│   │       └── apache.go           # Implementasi Apache2 vhost & proxy_fcgi
│   ├── http/
│   │   ├── handler/                # HTTP Handlers (Dashboard, Domains, File Manager, DB, Users, dll.)
│   │   ├── handler/                # HTTP Handlers (Dashboard, Domains, File Manager, DB, Users, dll.)
│   │   ├── middleware/             # Security headers, RequireAuth, RequireRole
│   │   └── router.go               # Routing Chi HTTP
│   ├── model/                      # Struct data Go (User, Domain, Quota, Database, Settings)
│   ├── model/                      # Struct data Go (User, Domain, Quota, Database, Settings)
│   └── storage/
│       ├── flock.go                # Interface advisory file lock
│       ├── flock_unix.go           # Implementasi flock Linux (unix.Flock)
│       ├── flock_windows.go        # Implementasi Windows LockFileEx
│       ├── json_store.go           # Atomic JSON flat-file storage engine
│       └── repository.go           # Repository layer
├── web/
│   ├── embed.go                    # embed.FS bundle untuk template & assets
│   ├── static/
│   │   ├── css/style.css           # Styling Fira Sans & JetBrains Mono
│   │   └── img/logo.jpg            # Logo resmi SEINTECH (Sekadau Innovation Technology)
│   │   ├── css/style.css           # Styling Fira Sans & JetBrains Mono
│   │   └── img/logo.jpg            # Logo resmi SEINTECH (Sekadau Innovation Technology)
│   └── templates/
│       ├── layout.html             # Base layout Tailwind CSS (Light Mode)
│       ├── dashboard.html          # Halaman statistik & service status
│       ├── domains.html            # Manajemen virtual host
│       ├── filemanager.html        # Web file manager & code editor (chroot jailed)
│       ├── databases.html          # Manajemen MySQL, PostgreSQL, & SSO phpMyAdmin
│       ├── filemanager.html        # Web file manager & code editor (chroot jailed)
│       ├── databases.html          # Manajemen MySQL, PostgreSQL, & SSO phpMyAdmin
│       ├── php.html                # Multi-PHP engine controller
│       ├── ssl.html                # Manajemen sertifikat SSL Let's Encrypt
│       ├── users.html              # Manajemen RBAC & Resource Quotas
│       ├── services.html           # Manajemen packages & system services
│       ├── settings.html           # Pengaturan port & hostname
│       ├── ssl.html                # Manajemen sertifikat SSL Let's Encrypt
│       ├── users.html              # Manajemen RBAC & Resource Quotas
│       ├── services.html           # Manajemen packages & system services
│       ├── settings.html           # Pengaturan port & hostname
│       └── login.html              # Halaman login
├── Makefile
├── go.mod
└── README.md
```

---

## 🛠️ Panduan Kompilasi & Menjalankan

### 1. Menjalankan Mode Development
Untuk menguji panel di mesin lokal tanpa memodifikasi `/var/lib/seintech`:
```bash
go run ./cmd/seintech -dev -port 8443
```
Panel akan otomatis membuat folder data lokal (`./data`, `./vhosts`, `./ssl`) dan meng-generate sertifikat TLS self-signed untuk akses HTTPS:
Buka di browser: `https://localhost:8443`  
Default kredensial:
- **Username:** `admin`
- **Password:** `admin123`

### 2. Kompilasi Standalone Binary (Linux Production)
Gunakan `Makefile`:
```bash
make build-linux
```
Atau via perintah Go langsung:
```bash
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/seintech ./cmd/seintech
```

### 3. Deploy sebagai Systemd Service pada Linux (Debian/Ubuntu/RHEL)
Jalankan skrip instalasi langsung sebagai `root`:
```bash
./deploy/install.sh
```
Atau manual:
```bash
cp bin/seintech /usr/local/bin/seintech
cp deploy/systemd/seintech.service /etc/systemd/system/
systemctl daemon-reload
systemctl enable --now seintech
```
Cek status layanan:
```bash
systemctl status seintech
journalctl -u seintech -f
```
