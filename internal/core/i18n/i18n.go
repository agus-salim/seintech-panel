package i18n

import (
	"fmt"
	"net/http"
	"strings"
)

const (
	LangEN = "en"
	LangID = "id"
)

// NormalizeLang ensures the language code is either "en" or "id".
func NormalizeLang(l string) string {
	l = strings.ToLower(strings.TrimSpace(l))
	if l == LangID {
		return LangID
	}
	return LangEN
}

// GetLangFromRequest inspects cookie, query parameter, or settings to determine the active language.
func GetLangFromRequest(r *http.Request, defaultLangs ...string) string {
	// 1. Query parameter override ?lang=en or ?lang=id
	if q := r.URL.Query().Get("lang"); q != "" {
		return NormalizeLang(q)
	}

	// 2. Cookie override
	if c, err := r.Cookie("seintech_lang"); err == nil && c.Value != "" {
		return NormalizeLang(c.Value)
	}

	// 3. Fallback to passed default (e.g. from PanelSettings)
	defaultLang := LangEN
	if len(defaultLangs) > 0 && defaultLangs[0] != "" {
		defaultLang = defaultLangs[0]
	}
	return NormalizeLang(defaultLang)
}

// T translates an English phrase to the target language.
// If target is "en" or not found, it returns the phrase formatted with args.
func T(lang, phrase string, args ...any) string {
	lang = NormalizeLang(lang)
	res := phrase

	if lang == LangID {
		if val, ok := idTranslations[phrase]; ok {
			res = val
		} else if val, ok := idTranslations[strings.TrimSpace(phrase)]; ok {
			res = val
		}
	}

	if len(args) > 0 {
		return fmt.Sprintf(res, args...)
	}
	return res
}

// idTranslations holds the English to Indonesian mappings
var idTranslations = map[string]string{
	// Navigation & System Titles
	"Dashboard":                   "Dasbor",
	"Virtual Hosts & Web":         "Virtual Host & Web",
	"Virtual Hosts":               "Virtual Host",
	"Virtual Hosts & Web Routing": "Virtual Host & Perutean Web",
	"Web File Manager":            "Pengelola Berkas Web",
	"Web File Manager & Code Editor": "Pengelola Berkas Web & Editor Kode",
	"File Manager":                "Pengelola Berkas",
	"Databases":                   "Basis Data",
	"Multi-Database Engine Provisioning": "Penyediaan Mesin Multi-Basis Data",
	"Users & Quotas":              "Pengguna & Kuota",
	"User, Reseller & RBAC Administration": "Administrasi Pengguna, Reseller & RBAC",
	"PHP Engines":                 "Mesin PHP",
	"Multi-PHP Version Engine & FPM Pools": "Mesin Multi-Versi PHP & Pool FPM",
	"SSL & Certificates":          "SSL & Sertifikat",
	"Automated Let's Encrypt & Custom SSL": "Let's Encrypt Otomatis & Kustom SSL",
	"Services & Packages":         "Layanan & Paket",
	"Service Manager & Package Installer": "Manajer Layanan & Pemasang Paket",
	"Panel Settings":              "Pengaturan Panel",
	"Panel & Server Configuration": "Konfigurasi Panel & Server",
	"System Admin":                "Admin Sistem",
	"Node Host":                   "Host Node",
	"Online":                      "Online",
	"Logout":                      "Keluar",
	"Systems Panel":               "Panel Sistem",

	// Common Actions & Buttons
	"Actions":                     "Tindakan",
	"Action":                      "Tindakan",
	"Save":                        "Simpan",
	"Save Changes":                "Simpan Perubahan",
	"Saving...":                   "Menyimpan...",
	"Saved!":                      "Tersimpan!",
	"Cancel":                      "Batal",
	"Close":                       "Tutup",
	"Delete":                      "Hapus",
	"Edit":                        "Edit",
	"Create":                      "Buat",
	"Apply":                       "Terapkan",
	"Back":                        "Kembali",
	"Refresh":                     "Muat Ulang",
	"Download":                    "Unduh",
	"Upload":                      "Unggah",
	"Install":                     "Pasang",
	"Uninstall":                   "Copot",
	"Restart":                     "Mulai Ulang",
	"Stop":                        "Hentikan",
	"Start":                       "Jalankan",
	"Confirm":                     "Konfirmasi",
	"Permanent":                   "Permanen",
	"Filter":                      "Saring",
	"Search":                      "Cari",
	"Status":                      "Status",
	"Active":                      "Aktif",
	"Inactive":                    "Tidak Aktif",
	"Enabled":                     "Aktif",
	"Disabled":                    "Nonaktif",
	"Default":                     "Bawaan",
	"Site:":                       "Situs:",
	"Total":                       "Total",
	"Used":                        "Digunakan",
	"Free":                        "Tersedia",
	"Unlimited":                   "Tanpa Batas",

	// Confirmation & Modals
	"Confirm Deletion":            "Konfirmasi Penghapusan",
	"Are you sure you want to proceed with this action?": "Apakah Anda yakin ingin melanjutkan tindakan ini?",
	"Uninstall Service":           "Copot Layanan",
	"Installing Package...":       "Memasang Paket Layanan...",
	"This may take a few minutes. Please do not close this page.": "Proses ini memerlukan waktu beberapa saat. Mohon jangan tutup halaman ini.",
	"Activating Let's Encrypt SSL": "Mengaktifkan SSL Let's Encrypt",
	"Renewing Let's Encrypt SSL":   "Memperbarui SSL Let's Encrypt",
	"SSL Activated Successfully! 🎉": "SSL Berhasil Diaktifkan! 🎉",
	"Connecting to APT repository...": "Menghubungkan ke repositori APT...",

	// Virtual Hosts / Domains
	"Create Virtual Host":         "Buat Virtual Host",
	"New Virtual Host":            "Virtual Host Baru",
	"Domain Name":                 "Nama Domain",
	"Web Server Engine":           "Mesin Web Server",
	"PHP Version":                 "Versi PHP",
	"Routing Mode":                "Mode Perutean",
	"Standard PHP-FPM":            "Standar PHP-FPM",
	"Static HTML / Assets Only":   "Statis HTML / Aset Saja",
	"Reverse Proxy (Node/Go/Python)": "Reverse Proxy (Node/Go/Python)",
	"Proxy Target URL":            "URL Target Proxy",
	"Owner Account":               "Akun Pemilik",
	"Dedicated Linux User":        "Pengguna Linux Terisolasi",
	"Document Root":               "Direktori Dokumen (Document Root)",
	"Issue Free Let's Encrypt SSL": "Terbitkan SSL Gratis Let's Encrypt",
	"Create Site Now":             "Buat Situs Sekarang",
	"Manage Domain":               "Kelola Domain",
	"Virtual Host Actions":        "Aksi Virtual Host",
	"Edit Virtual Host":           "Edit Virtual Host",
	"View Error Logs":             "Lihat Log Kesalahan",
	"View Access Logs":            "Lihat Log Akses",
	"Setup Database":              "Siapkan Basis Data",
	"Delete Virtual Host":         "Hapus Virtual Host",
	"No virtual hosts found.":     "Belum ada virtual host yang terdaftar.",
	"Create your first virtual host to begin hosting websites.": "Buat virtual host pertama Anda untuk mulai menayangkan situs web.",
	"Domain name is already registered": "Nama domain sudah terdaftar",
	"This domain name is already registered in the panel!": "Nama domain ini sudah terdaftar di panel!",
	"Please choose a different domain name.": "Silakan gunakan nama domain yang lain.",
	"Domain name is required":     "Nama domain wajib diisi",
	"Virtual host created successfully": "Virtual host berhasil dibuat",
	"Virtual host deleted successfully": "Virtual host berhasil dihapus",
	"Deletion cancelled. Please type 'confirm' to delete.": "Penghapusan dibatalkan. Harap ketik 'confirm' atau 'konfirmasi' untuk menghapus.",
	"Access denied: You do not have permission to delete this domain": "Akses ditolak: Anda tidak memiliki izin untuk menghapus domain ini.",
	"Confirm Virtual Host Deletion": "Konfirmasi Hapus Virtual Host",
	"This action is permanent and cannot be undone.": "Tindakan ini permanen dan tidak dapat dibatalkan.",
	"Type the word":               "Ketik kata",
	"to proceed:":                 "untuk melanjutkan:",
	"SSL Status":                  "Status SSL",
	"Active SSL":                  "SSL Aktif",
	"No SSL":                      "Tanpa SSL",
	"Auto-Renew":                  "Perpanjang Otomatis",

	// Databases
	"Create Database":             "Buat Basis Data",
	"Database Name":               "Nama Basis Data",
	"Database User":               "Pengguna Basis Data",
	"DB User":                     "Pengguna DB",
	"Password":                    "Kata Sandi",
	"Host":                        "Host",
	"Import SQL Database":         "Impor Basis Data SQL",
	"Export SQL Database":         "Ekspor Basis Data SQL",
	"Change DB Password":          "Ubah Kata Sandi DB",
	"Delete Database":             "Hapus Basis Data",
	"Database target:":            "Basis data tujuan:",
	"Select .sql or .sql.gz file": "Pilih berkas .sql atau .sql.gz",
	"Upload & Import":             "Unggah & Impor",
	"No databases created yet.":   "Belum ada basis data yang dibuat.",

	// Users & Quotas
	"Create Account":              "Buat Akun",
	"Create New Account":          "Buat Akun Baru",
	"Username":                    "Nama Pengguna",
	"Account Role":                "Peran Akun",
	"Administrator (Full Access)": "Administrator (Akses Penuh)",
	"Reseller (Can Manage Users)": "Reseller (Bisa Kelola Pengguna)",
	"End User (Single Site)":      "Pengguna Akhir (Satu Situs)",
	"Disk Quota (MB)":             "Kuota Disk (MB)",
	"Bandwidth Quota (MB)":        "Kuota Bandwidth (MB)",
	"Max Domains":                 "Maksimal Domain",
	"Max Databases":               "Maksimal Basis Data",
	"Edit Resource Quotas":        "Edit Kuota Sumber Daya",
	"Save Quotas":                 "Simpan Kuota",
	"Delete User Account":         "Hapus Akun Pengguna",
	"Are you sure you want to delete user": "Apakah Anda yakin ingin menghapus pengguna",

	// File Manager
	"New File":                    "Berkas Baru",
	"New Directory":               "Direktori Baru",
	"New Folder":                  "Folder Baru",
	"Create New File":             "Buat Berkas Baru",
	"Create New Directory":        "Buat Direktori Baru",
	"Reset Permissions (755/644)": "Pulihkan Izin Default (755/644)",
	"Drop files here to upload":   "Lepaskan berkas di sini untuk mengunggah",
	"Files will be automatically uploaded to:": "Berkas akan otomatis diunggah ke direktori:",
	"Extract Archive":             "Ekstrak Arsip",
	"Extract Now":                 "Ekstrak Sekarang",
	"Destination Directory":       "Direktori Tujuan",
	"Compress Files":              "Kompres Berkas",
	"Compress Now":                "Kompres Sekarang",
	"Archive Name":                "Nama Berkas Arsip",
	"Bulk Delete":                 "Hapus Masal",
	"Delete Selected":             "Hapus Terpilih",
	"Permanently Delete?":         "Hapus Permanen?",
	"items will be permanently removed from disk.": "item akan dihapus secara permanen dari server.",
	"Yes, Delete All":             "Ya, Hapus Semua",
	"Name":                        "Nama",
	"Size":                        "Ukuran",
	"Owner":                       "Pemilik",
	"Access":                      "Izin Akses",
	"Changed":                     "Diubah",
	"Parent Directory":            "Direktori Induk",
	"This directory is empty.":    "Direktori ini masih kosong.",
	"Drag & drop files here to upload automatically.": "Tarik & lepaskan berkas ke sini untuk mengunggah otomatis.",
	"Uploading File...":           "Mengunggah Berkas...",
	"Processing...":               "Memproses...",
	"Upload Complete!":            "Unggahan Selesai!",
	"Rename Item":                 "Ubah Nama Berkas/Folder",
	"New Name":                    "Nama Baru",
	"Change Permissions (chmod)":  "Ubah Izin Akses (chmod)",
	"Octal Mode (e.g. 0755, 0644)": "Mode Oktal (misal 0755, 0644)",

	// PHP Engines
	"PHP Engine Management":       "Manajemen Mesin PHP",
	"Installed Versions & Pools":  "Versi Terpasang & Pool FPM",
	"Configure php.ini":           "Konfigurasi php.ini",
	"Popular Settings":            "Pengaturan Populer",
	"Full php.ini Editor":         "Editor php.ini Lengkap",
	"After saving, PHP-FPM service will automatically reload": "Setelah disimpan, service PHP-FPM akan otomatis dimuat ulang",
	"Save & Apply":                "Simpan & Terapkan",
	"PHP Extensions":              "Ekstensi PHP",
	"Restart Service":             "Mulai Ulang Layanan",
	"Reload Service":              "Muat Ulang Layanan",
	"Directives":                  "Direktif",
	"Raw php.ini":                 "php.ini Mentah",
	"Save & Restart":              "Simpan & Mulai Ulang",

	// Services & Packages
	"Installed Services & System Packages": "Layanan Sistem & Paket Terpasang",
	"Install New Package / Service":        "Pasang Paket / Layanan Baru",
	"Select a system service package to install on the server": "Pilih paket layanan sistem yang ingin dipasang di server",
	"Web Servers":                 "Server Web",
	"Database Servers":            "Server Basis Data",
	"Database Tools":              "Peralatan Basis Data",
	"PHP-FPM Engines":             "Mesin PHP-FPM",
	"System Tools & Utilities":    "Peralatan & Utilitas Sistem",
	"No Web Server installed.":    "Tidak ada Web Server yang terpasang.",
	"No Database Server installed.": "Tidak ada Server Basis Data yang terpasang.",

	// Settings
	"Server Defaults & Configuration": "Pengaturan Bawaan & Konfigurasi Server",
	"Default Web Server":          "Web Server Bawaan",
	"Default PHP Version":         "Versi PHP Bawaan",
	"Security Headers":            "Header Keamanan HTTP",
	"Enable modern security headers (HSTS, CSP, X-Frame-Options)": "Aktifkan header keamanan modern (HSTS, CSP, X-Frame-Options)",
	"Panel Language":              "Bahasa Panel",
	"Panel Access Hostnames & SSL": "Domain Akses Panel & SSL",
	"Add Panel Domain":            "Tambah Domain Akses Panel",
	"Edit Panel Domain":           "Edit Domain Akses Panel",
	"Delete Panel Domain":         "Hapus Domain Akses Panel",
	"Hostname / Domain":           "Hostname / Domain",
	"HTTPS Port":                  "Port HTTPS",
	"Save Server Configuration":   "Simpan Konfigurasi Server",
	"English (US)":                "English (US)",
	"Indonesian (Bahasa Indonesia)": "Bahasa Indonesia (ID)",

	// Login & Auth
	"Sign in to your control panel": "Masuk ke panel kontrol Anda",
	"Sign In":                     "Masuk",
	"Invalid username or password": "Nama pengguna atau kata sandi tidak valid",
}
