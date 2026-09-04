<div align="center">

# ⚡ INDOGARO CORE SERVICE ⚡
### Enterprise High-Performance Headless Multiplexer & Proxy Engine for Android

[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](https://www.gnu.org/licenses/gpl-3.0)
[![Indogaro License](https://img.shields.io/badge/License-Indogaro--v1.0-orange.svg)](INDOGARO-LICENSE.md)
[![Android Target](https://img.shields.io/badge/Target%20SDK-28%20(Pure%20Native%20Bypass)-brightgreen.svg)]()
[![Arch Support](https://img.shields.io/badge/Arch-ARM64%20|%20aarch64-red.svg)]()
[![Zero I/O Log](https://img.shields.io/badge/I%2FO%20Engine-DevNull%20Silence%20Mode-yellowgreen.svg)]()

</div>

---

## 📑 Daftar Isi
1. [Tentang Indogaro Core Service](#-tentang-indogaro-core-service)
2. [Fitur & Keunggulan Arsitektur](#-fitur--keunggulan-arsitektur)
3. [Alokasi Subnet & Port Internal](#-alokasi-subnet--port-internal)
4. [Instalasi & Panduan Aktivasi](#-instalasi--panduan-aktivasi)
   - [Metode 1: Aktivasi via ADB (Non-Root / Rekomendasi)](#metode-1-aktivasi-via-adb-non-root--rekomendasi)
   - [Metode 2: Aktivasi via Termux (Perangkat Root)](#metode-2-aktivasi-via-termux-perangkat-root)
   - [Metode 3: Bypass Optimasi Baterai Ekstrem (OEM Xiaomi, Samsung, Oppo, Vivo)](#metode-3-bypass-optimasi-baterai-ekstrem-oem)
5. [Panduan Integrasi ke v2rayNG](#-panduan-integrasi-ke-v2rayng)
   - [Skema Routing SOCKS5](#1-skema-routing-socks5)
   - [Skema Routing HTTP Custom Proxy](#2-skema-routing-http-custom-proxy)
   - [Verifikasi Status Koneksi](#3-verifikasi-status-koneksi)
6. [Struktur File Konfigurasi (.env & assets)](#-struktur-file-konfigurasi-env--assets)
7. [Mekanisme Self-Healing & Watchdog](#-mekanisme-self-healing--watchdog)
8. [Panduan Build Sendiri (Local Compilation)](#-panduan-build-sendiri-local-compilation)
9. [Ketentuan Lisensi, Copyright & Forking (IndogaroLicense-v1.0)](#-ketentuan-lisensi-copyright--forking)

---

## 📖 Tentang Indogaro Core Service

**Indogaro Core Service** (`com.indogaro.service`) adalah engine background headless tingkat rendah (*low-level headless daemon*) yang dibangun menggunakan kombinasi **Go Native Supervisor** dan **Android Foreground Service System**.

Engine ini dirancang khusus untuk menangani proses tunneling jaringan yang membutuhkan stabilitas tinggi, nol jitter (*zero jitter*), latensi minimal, dan kebal terhadap pembersihan paksa oleh sistem operasi Android (*anti-OOM kill*).

---

## 🚀 Fitur & Keunggulan Arsitektur

* **Pure Headless Operation**: Tidak memuat UI Activity, tidak memakan RAM untuk rendering UI, dan tidak memunculkan icon aplikasi yang mengotori App Drawer / Homescreen.
* **Zero I/O Silence Mode (`> /dev/null 2>&1`)**: Seluruh output proses sub-biner dibuang ke *null channel* secara native sehingga memangkas pemakaian CPU dan mencegah keausan storage akibat logcat berlebihan.
* **TargetSdk 28 Standalone**: Mengeksekusi binary Linux langsung dari internal sandbox storage tanpa restriksi SELinux W^X Android 10+.
* **TCP Socket Deadlock Watchdog**: Go supervisor secara otomatis mem-ping socket lokal setiap 15 detik. Jika terjadi port-freeze / deadlock, daemon langsung me-reload instance dalam milidetik.
* **Persistent Low-Level WakeLock**: Menjaga CPU tetap responsif saat layar HP mati tanpa menguras daya baterai secara agresif.
* **Multi-Stage Auto Re-Spawn**:
  * **Saat Boot/Reboot**: Otomatis hidup sendiri via `BOOT_COMPLETED` & `QUICKBOOT_POWERON`.
  * **Saat Update APK**: Otomatis hidup kembali tanpa intervensi via `MY_PACKAGE_REPLACED`.
* **Zero-Conflict Keystore Pipeline**: Auto-increment versioning terpadu yang menjamin update berjalan mulus seumur hidup.

---

## 🌐 Alokasi Subnet & Port Internal

Daemon secara otomatis mengamankan dan mengalokasikan port loopback internal:

| Endpoint Loopback | Protokol / Tipe | Fungsi | Status Watchdog |
| :--- | :--- | :--- | :--- |
| **`127.0.0.3:2007`** | TCP / SOCKS5 / HTTP | Primary High-Speed Inbound Proxy | Dipantau Otomatis (Active Ping) |
| **`127.0.0.3:2008`** | TCP / SOCKS5 / HTTP | Secondary Multiplexer / Failover | Dipantau Otomatis (Active Ping) |
| **`127.0.0.1:9090`** | TCP / REST | Internal Health & Control API (Opsional) | Internal Memory Check |

---

## 📲 Instalasi & Panduan Aktivasi

### 1. Unduh APK
Unduh file APK rilis resmi terbaru (`indogaro-service-v*.apk`) dari halaman [Releases](../../releases).

### 2. Pasang APK
