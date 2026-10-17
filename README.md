# Seintech Hosting Panel 🚀

**Seintech Hosting Panel** adalah web hosting control panel modern berbasis **single binary Go** yang ultra-ringan, modular, dan tangguh. Dirancang khusus untuk berjalan sebagai `systemd` service pada sistem operasi Linux (Debian, Ubuntu) tanpa memerlukan database eksternal untuk konfigurasi internalnya.

Repositori Resmi: **[https://github.com/seintechid/panel](https://github.com/seintechid/panel)**

---

## 🌟 Arsitektur & Keunggulan Utama

1. **Standalone Single Binary Go (`embed.FS`):**
   - Seluruh static assets (CSS, JS, logo) dan Go HTML templates dibundle langsung ke dalam satu binary menggunakan fitur Go `embed.FS`.
   - Tidak memerlukan dependensi Node.js, runtime Python, atau interpreter di host server.
   
2. **Penyimpanan Flat-File JSON Berintegritas Tinggi:**
   - Menyimpan seluruh konfigurasi di `/var/lib/seintech/data/*.json`.
   - **Dual-Locking:** Dilindungi oleh `sync.RWMutex` (in-process concurrency safety) dan OS-level `flock` (advisory file locking) untuk mencegah race condition antar-proses.
   - **Atomic Write:** Menggunakan strategi penulisan ke temporary file kemudian `os.Rename` untuk mencegah kerusakan data jika terjadi pemadaman mendadak.

3. **Multi-Webserver Management (Nginx & Apache2):**
   - Abstraksi interface `WebServer` yang mengendalikan **Nginx** (High-Performance Engine / Reverse Proxy) dan **Apache2** (MPM Event & PHP-FPM).
   - Dilengkapi template generator vhost untuk reverse proxy, static site, dan PHP-FPM socket router.

4. **Multi-PHP Version Engine (7.4 s/d 8.5):**
   - Mendukung eksekusi pool PHP-FPM individual (`/run/php/phpX.Y-fpm.sock`).
   - Pengaturan `php.ini` dasar per-domain (`upload_max_filesize`, `memory_limit`, `max_execution_time`).

5. **Multi-Database Engine:**
   - Mendukung **MariaDB/MySQL** dan **PostgreSQL**.
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

---

## 📁 Struktur Direktori Proyek

```text
seintech-panel/
├── bin/
│   ├── seintech                    # Precompiled Linux standalone binary (x86_64)
│   ├── seintech-linux-amd64        # Salinan binary deployment Linux x86_64
│   └── seintech.exe                # Binary Windows (testing)
├── cmd/
│   └── seintech/
│       └── main.go                 # Entrypoint aplikasi, CLI flags, bootstrap admin & graceful shutdown
├── configs/
│   └── schema/                     # Skema flat-file JSON
│       ├── users.json
│       ├── domains.json
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
│   │   ├── auth/                   # Manajemen sesi & enkripsi bcrypt
│   │   ├── database/               # Driver provisioning MariaDB & PostgreSQL
│   │   ├── filemanager/            # Operasi file, kompresi, ZipSlip guard & chroot
│   │   ├── php/                    # Manajemen engine PHP 7.4-8.5 & FPM pools
│   │   ├── ssl/                    # ACME client, self-signed generator & auto-renewal
│   │   ├── system/                 # Metrik Linux, kuota, service manager & background jobs
│   │   └── webserver/              # Interface WebServer, Nginx & Apache2 drivers
│   ├── http/
│   │   ├── handler/                # HTTP Handlers (Dashboard, Domains, File Manager, DB, Users, dll.)
│   │   ├── middleware/             # Security headers, RequireAuth, RequireRole
│   │   └── router.go               # Routing Chi HTTP
│   ├── model/                      # Struct data Go (User, Domain, Quota, Database, Settings)
│   └── storage/                    # Dual-locking & atomic JSON flat-file storage engine
├── web/
│   ├── embed.go                    # embed.FS bundle untuk template & assets
│   ├── static/
│   │   ├── css/style.css           # Styling Fira Sans & JetBrains Mono
│   │   └── img/logo.jpg            # Logo resmi SEINTECH
│   └── templates/                  # Go HTML Templates (Tailwind UI)
├── Makefile
├── go.mod
└── README.md
```

---

## 🚀 Panduan Instalasi di Server Debian / Ubuntu (Production)

### 1. Prasyarat Server
- Sistem Operasi: **Debian 11 (Bullseye), Debian 12 (Bookworm), Debian 13 (Trixie)** atau **Ubuntu 20.04/22.04/24.04 LTS**.
- Hak akses: **root** (atau user dengan akses `sudo`).
- Port yang perlu dibuka di firewall:
  - `8443` : Dashboard Panel (HTTPS)
  - `80`   : HTTP / Let's Encrypt Verification
  - `443`  : HTTPS Web Server
  - `22`   : SSH Server

---

### 2. Langkah Instalasi Otomatis (Direkomendasikan)

Masuk ke server Debian Anda via SSH, kemudian jalankan langkah berikut:

#### Langkah A: Perbarui paket dasar server
```bash
apt update -y && apt install -y git curl wget ca-certificates
```

#### Langkah B: Clone repositori resmi
```bash
git clone https://github.com/seintechid/panel.git /opt/seintech-panel
```

#### Langkah C: Jalankan skrip installer
```bash
cd /opt/seintech-panel
chmod +x deploy/install.sh
./deploy/install.sh
```

Skrip installer otomatis melakukan:
1. Pembuatan direktori aplikasi (`/var/lib/seintech/data`, `/var/lib/seintech/ssl`, `/var/www`, dll).
2. Konfigurasi repositori resmi PHP (Ondrej Sury PPA / packages.sury.org).
3. Pemasangan binary `/usr/local/bin/seintech` dari precompiled binary yang sudah ada di repo.
4. Pendaftaran dan aktivasi `systemd` unit `/etc/systemd/system/seintech.service`.
5. Menjalankan service Seintech Panel di latar belakang.

---

### 3. Konfigurasi Firewall (UFW)

Jika Anda menggunakan UFW (Uncomplicated Firewall) pada Debian/Ubuntu, buka port yang dibutuhkan:

```bash
ufw allow 8443/tcp comment "Seintech Panel HTTPS"
ufw allow 80/tcp comment "HTTP Web"
ufw allow 443/tcp comment "HTTPS Web"
ufw allow 22/tcp comment "SSH"
ufw reload
```

---

### 4. Mengakses Web Dashboard

Setelah instalasi selesai, buka browser Anda:

- **Alamat URL:** `https://<IP_SERVER_ANDA>:8443`  
  *(Contoh: `https://192.168.1.100:8443` atau `https://203.0.113.10:8443`)*
- **Peringatan SSL:** Karena panel menggunakan SSL self-signed pada bootstrap awal, browser akan menampilkan peringatan keamanan. Klik **Advanced / Tingkat Lanjut** lalu pilih **Proceed to ... (unsafe)** untuk melanjutkan.
- **Kredensial Default:**
  - **Username:** `admin`
  - **Password:** `admin123`

> [!IMPORTANT]
> Segera ganti kata sandi akun default setelah pertama kali login melalui menu **Users** atau **Settings** demi keamanan server Anda.

---

### 5. Manajemen Layanan (Systemd)

Gunakan perintah `systemctl` untuk mengelola layanan Seintech Panel:

```bash
# Memeriksa status layanan
systemctl status seintech

# Memantau log aplikasi secara real-time
journalctl -u seintech -f

# Restart layanan
systemctl restart seintech

# Menghentikan layanan
systemctl stop seintech

# Memulai kembali layanan
systemctl start seintech
```

---

### 6. Cara Memperbarui / Update Panel

Jika ada pembaruan fitur terbaru di repositori resmi GitHub, perbarui panel dengan langkah ringkas:

```bash
cd /opt/seintech-panel
git pull
systemctl stop seintech
cp -f bin/seintech /usr/local/bin/seintech
systemctl start seintech
```

> [!TIP]
> Service `seintech` perlu dihentikan (`stop`) terlebih dahulu sebelum menyalin biner agar tidak terjadi error *Text file busy* pada Linux. Alternatifnya, Anda juga bisa langsung menjalankan `./deploy/install.sh`.

---

## 🛠️ Opsi Instalasi Lanjutan & Development

### Kompilasi Manual dari Source (Arsitektur ARM64 / Non-x86)
Jika server Anda menggunakan prosesor arsitektur ARM64 (seperti Raspberry Pi atau Oracle Cloud Ampere A1) atau Anda ingin mengompilasi binary sendiri:

1. Pasang compiler Go di server Debian:
   ```bash
   apt install -y golang-go
   ```
2. Kompilasi dan pasang binary:
   ```bash
   cd /opt/seintech-panel
   go build -ldflags="-s -w" -o /usr/local/bin/seintech ./cmd/seintech
   chmod 755 /usr/local/bin/seintech
   cp deploy/systemd/seintech.service /etc/systemd/system/seintech.service
   systemctl daemon-reload
   systemctl enable --now seintech
   ```

### Menjalankan Mode Development Lokal
Untuk menguji dan mengembangkan panel di laptop/komputer lokal tanpa mengganggu sistem operasi host:
```bash
go run ./cmd/seintech -dev -port 8443
```
Panel akan otomatis membuat folder sandbox lokal (`./data`, `./vhosts`, `./ssl`) dan meng-generate sertifikat TLS self-signed untuk pengujian di `https://localhost:8443`.
