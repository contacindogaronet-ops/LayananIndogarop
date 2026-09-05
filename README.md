<div align="center">

# ⚡ INDOGARO CORE SERVICE ⚡
### Enterprise High-Performance Headless Multiplexer & Proxy Carrier for Android

[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](LICENSE)
[![Indogaro License](https://img.shields.io/badge/License-Indogaro--v1.0-orange.svg)](INDOGARO-LICENSE.md)
[![Android Target](https://img.shields.io/badge/Target%20SDK-28%20(Pure%20Native%20Bypass)-brightgreen.svg)](#-arsitektur--spesifikasi-teknis)
[![RAM Capacity](https://img.shields.io/badge/RAM%20Pool-300MB%20Resilience-purple.svg)](#-alokasi-memori--ketahanan-traffic-300mb-ram)
[![Arch Support](https://img.shields.io/badge/Arch-ARM64%20%7C%20aarch64-red.svg)](#-arsitektur--spesifikasi-teknis)
[![Zero I/O Log](https://img.shields.io/badge/I%2FO%20Engine-DevNull%20Silence%20Mode-yellowgreen.svg)](#-fitur--keunggulan-arsitektur)

</div>

---

## 📑 Daftar Isi
1. [Tentang Indogaro Core Service](#-tentang-indogaro-core-service)
2. [Fitur & Keunggulan Arsitektur](#-fitur--keunggulan-arsitektur)
3. [Alokasi Memori & Ketahanan Traffic (300MB RAM)](#-alokasi-memori--ketahanan-traffic-300mb-ram)
4. [Alokasi Subnet & Port Internal](#-alokasi-subnet--port-internal)
5. [Instalasi & Panduan Aktivasi](#-instalasi--panduan-aktivasi)
   - [Metode 1: Aktivasi via ADB (Rekomendasi Utama)](#metode-1-aktivasi-via-adb-rekomendasi-utama)
   - [Metode 2: Aktivasi via Termux / Shell Lokal (Root)](#metode-2-aktivasi-via-termux--shell-lokal-root)
   - [Metode 3: Bypass Optimasi Baterai Ekstrem (Xiaomi HyperOS, Samsung, Oppo, Vivo)](#metode-3-bypass-optimasi-baterai-ekstrem-xiaomi-hyperos-samsung-oppo-vivo)
6. [Panduan Integrasi Tunnel (v2rayNG, Clash, dll)](#-panduan-integrasi-tunnel-v2rayng-clash-dll)
   - [Skema Routing SOCKS5](#1-skema-routing-socks5)
   - [Skema Routing HTTP Custom Proxy](#2-skema-routing-http-custom-proxy)
   - [Verifikasi Status Koneksi](#3-verifikasi-status-koneksi)
7. [Mekanisme Auto-Update Tanpa Download Fisik Manual](#-mekanisme-auto-update-tanpa-download-fisik-manual)
8. [Arsitektur & Spesifikasi Teknis](#-arsitektur--spesifikasi-teknis)
9. [Panduan Build Sendiri (Local Compilation)](#-panduan-build-sendiri-local-compilation)
10. [Ketentuan Lisensi, Copyright & Forking](#-ketentuan-lisensi-copyright--forking)

---

## 📖 Tentang Indogaro Core Service

**Indogaro Core Service** (`com.indogaro.service`) adalah engine background headless tingkat rendah (*low-level headless daemon*) yang dibangun menggabungkan **Go Native Supervisor** dan **Android Persistent Foreground Service**.

Engine ini bertindak sebagai **penopang traffic internet di smartphone** yang menangani jutaan paket data, proses proxying/multiplexing, anti-deadlock port, serta kebal terhadap pembersihan paksa oleh sistem operasi Android (*anti-OOM kill*).

---

## 🚀 Fitur & Keunggulan Arsitektur

* **Pure Headless Daemon**: 100% tanpa Activity UI, tanpa merender layout yang memboroskan CPU, dan tidak memunculkan icon aplikasi yang mengotori Homescreen/App Drawer.
* **Autonomous In-Place Auto-Update**: Mengunduh dan memperbarui APK secara mandiri via Android FileProvider tanpa user harus membuka browser.
* **High-Throughput RAM Resilience (~300MB Pool)**: Mampu menampung serangan lonjakan traffic besar tanpa crash / force-close.
* **Zero I/O Silence Mode (`/dev/null`)**: Output sub-biner dibuang ke null channel sehingga mencegah ausnya storage dan menghindari pipe-buffer lock di kernel Linux.
* **TargetSdk 28 Linux Standalone**: Mengeksekusi biner Linux langsung dari internal sandbox storage tanpa restriksi SELinux W^X Android 10+.
* **TCP Socket Deadlock Watchdog**: Supervisor secara otomatis memeriksa soket loopback secara berkala. Jika terjadi port-freeze, daemon langsung me-reload instance dalam hitungan milidetik.
* **Persistent WakeLock & Auto-Respawn**:
  * **Saat Boot/Reboot**: Otomatis aktif via `BOOT_COMPLETED` & `QUICKBOOT_POWERON`.
  * **Saat Update Selesai**: Otomatis hidup kembali tanpa jeda via `MY_PACKAGE_REPLACED`.

---

## 🛡 Alokasi Memori & Ketahanan Traffic (300MB RAM)

Agar smartphone tidak mengalami *Low Memory Killer (LMK)* saat traffic jaringan membludak:
1. **`android:largeHeap="true"`**: Membuka batas heap ART/Dalvik VM Android ke batas maksimal hardware (>256MB - 512MB).
2. **`GOMEMLIMIT=280MiB`**: Menahan alokasi memori Go Runtime agar GC (*Garbage Collector*) tidak mengalami thrashing saat puluhan ribu paket data masuk bersamaan.
3. **`RLIMIT_NOFILE = 65535`**: Memperbesar batas file descriptor dan socket connections dari standar Android (1024) ke 65.535 socket simultan.
4. **`SetMaxThreads(10000)`**: Mengamankan thread pool sistem agar tidak terjadi socket starvation.

---

## 🌐 Alokasi Subnet & Port Internal

Daemon mengalokasikan dan menjaga stabilitas endpoint loopback internal:

| Endpoint Loopback | Protokol / Tipe | Fungsi | Status Watchdog |
| :--- | :--- | :--- | :--- |
| **`127.0.0.3:2007`** | TCP / SOCKS5 / HTTP | Primary High-Speed Inbound Proxy | Dipantau Otomatis (Active Probe) |
| **`127.0.0.3:2008`** | TCP / SOCKS5 / HTTP | Secondary Multiplexer / Failover | Dipantau Otomatis (Active Probe) |
| **`127.0.0.1:9090`** | TCP / REST | Internal Health & Control API (Opsional) | Internal Memory Check |

---

## 📲 Instalasi & Panduan Aktivasi

### 1. Unduh APK
Unduh file APK rilis resmi terbaru (`indogaro-service-v*.apk`) dari menu rilis project.

### 2. Pasang APK
Pasang APK seperti biasa menggunakan File Manager atau via ADB:
