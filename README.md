<div align="center">

# ⚡ INDOGARO CORE SERVICE ⚡
### Enterprise High-Performance Headless Multiplexer & Proxy Carrier for Android
*Developed & Engineered by Indogaro Group Autonomous Architecture System (`aiclo`)*

[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](LICENSE)
[![Indogaro Dual License](https://img.shields.io/badge/License-Indogaro--v1.0-orange.svg)](INDOGARO-LICENSE.md)
[![Android Target](https://img.shields.io/badge/Target%20SDK-28%20(Pure%20Native%20Bypass)-brightgreen.svg)](#-arsitektur--spesifikasi-teknis)
[![RAM Capacity](https://img.shields.io/badge/RAM%20Pool-300MB%20Resilience-purple.svg)](#-alokasi-memori--ketahanan-traffic-300mb-ram)
[![Arch Support](https://img.shields.io/badge/Arch-ARM64%20%7C%20aarch64-red.svg)](#-arsitektur--spesifikasi-teknis)
[![Zero I/O Log](https://img.shields.io/badge/I%2FO%20Engine-DevNull%20Silence%20Mode-yellowgreen.svg)](#-fitur--keunggulan-arsitektur)

</div>

---

## 📑 Daftar Isi
1. [Tentang Indogaro Core Service](#-tentang-indogaro-core-service)
2. [Fitur & Keunggulan Arsitektur](#-fitur--keunggulan-arsitektur)
3. [Alokasi Memori & Ketahanan Traffic (300MB Dedicated Buffer Pool)](#-alokasi-memori--ketahanan-traffic-300mb-dedicated-buffer-pool)
4. [Alokasi Subnet Loopback & Port Internal](#-alokasi-subnet-loopback--port-internal)
5. [Diagram Alir Sistem (Architecture Flow)](#-diagram-alir-sistem-architecture-flow)
6. [Instalasi & Panduan Aktivasi](#-instalasi--panduan-aktivasi)
   - [Metode 1: Aktivasi Instan via ADB (Rekomendasi)](#metode-1-aktivasi-instan-via-adb-rekomendasi)
   - [Metode 2: Aktivasi via Termux / Shell Lokal (Root Device)](#metode-2-aktivasi-via-termux--shell-lokal-root-device)
   - [Metode 3: Bypass Optimasi Baterai Ekstrem (HyperOS, OneUI, ColorOS)](#metode-3-bypass-optimasi-baterai-ekstrem-hyperos-oneui-coloros)
7. [Panduan Integrasi Tunnel (v2rayNG, Clash Meta, HTTP Custom, Nekobox)](#-panduan-integrasi-tunnel-v2rayng-clash-meta-http-custom-nekobox)
   - [Skema Routing SOCKS5 Inbound](#1-skema-routing-socks5-inbound)
   - [Skema Routing HTTP Custom Proxy](#2-skema-routing-http-custom-proxy)
   - [Verifikasi Status Konektivitas Engine](#3-verifikasi-status-konektivitas-engine)
8. [Mekanisme Autonomous In-Place Auto-Update](#-mekanisme-autonomous-in-place-auto-update)
9. [Arsitektur & Spesifikasi Teknis](#-arsitektur--spesifikasi-teknis)
10. [Panduan Build & Kompilasi Lokal](#-panduan-build--kompilasi-lokal)
11. [Ketentuan Lisensi, Copyright & Kontribusi](#-ketentuan-lisensi-copyright--kontribusi)

---

## 📖 Tentang Indogaro Core Service

**Indogaro Core Service** (`com.indogaro.service`) adalah engine *low-level headless proxy carrier* tingkat enterprise yang menggabungkan kekuatan **Go Native High-Performance Supervisor** dan **Android Persistent Foreground Service**.

Dirancang khusus sebagai infrastruktur penopang lalu lintas internet di lingkungan mobile, engine ini menangani puluhan ribu socket TCP/UDP simultan, multiplexing protokol, anti-deadlock watchdog, serta kebal terhadap pembunuhan paksa memori oleh sistem operasi Android (*anti-OOM / Low Memory Killer mitigation*).

---

## 🚀 Fitur & Keunggulan Arsitektur

* **100% Pure Headless Daemon**: Didesain tanpa Activity UI atau view components yang membebani CPU dan GPU. Tidak meninggalkan icon launcher yang mengotori App Drawer perangkat.
* **Autonomous In-Place Auto-Update**: Mengunduh, memverifikasi integritas payload, dan menginstal APK rilis terbaru secara mandiri melalui subsystem Android `FileProvider` tanpa intervensi browser eksternal.
* **High-Throughput RAM Resilience (~300MB Pool)**: Menangani lonjakan traffic multi-gigabit tanpa bottleneck dan terlindung dari crash *Out-Of-Memory*.
* **Zero I/O Silence Mode (`/dev/null`)**: Membuang log output sub-biner ke null channel untuk memperpanjang usia memori flash (UFS/eMMC) serta mencegah pipe buffer deadlock pada Linux kernel.
* **TargetSdk 28 Linux Standalone Bypass**: Mengeksekusi binary native Linux langsung dari direct sandbox storage tanpa terhambat restriksi SELinux W^X pada Android 10+.
* **Active TCP Socket Watchdog**: Supervisor native memeriksa ketersediaan port loopback secara real-time. Jika terjadi freeze atau socket drop, engine di-respawn dalam hitungan sub-detik.
* **Persistent WakeLock & Auto-Respawn Matrix**:
  * **System Boot Engine**: Otomatis aktif saat perangkat menyala (`BOOT_COMPLETED`, `QUICKBOOT_POWERON`).
  * **Hot Reload on Update**: Otomatis hidup kembali tanpa jeda via receiver `MY_PACKAGE_REPLACED`.

---

## 🛡 Alokasi Memori & Ketahanan Traffic (300MB Dedicated Buffer Pool)

Untuk memastikan transmisi paket data berkecepatan tinggi tanpa hambatan sistem:
1. **`android:largeHeap="true"`**: Memperluas heap virtual Dalvik/ART hingga batas maksimal hardware perangkat (>256MB–512MB).
2. **`GOMEMLIMIT=280MiB`**: Mengunci ambang batas runtime GC Go untuk mencegah *Garbage Collector thrashing* saat memproses puluhan ribu stream data.
3. **`RLIMIT_NOFILE = 65535`**: Menggandakan batas file descriptor dari default Android (1024) ke 65.535 socket simultan.
4. **`runtime.SetMaxThreads(10000)`**: Menjamin ketersediaan thread pool sistem operasi tanpa risiko socket starvation.

---

## 🌐 Alokasi Subnet Loopback & Port Internal

Daemon menginisialisasi dan memantau endpoint loopback berikut secara eksklusif:

| Endpoint Loopback | Protokol / Tipe | Peruntukan Fungsional | Status Watchdog |
| :--- | :--- | :--- | :--- |
| **`127.0.0.3:2007`** | TCP / SOCKS5 / HTTP | Primary High-Speed Inbound Proxy | Dipantau Aktif (Active Health Probe) |
| **`127.0.0.3:2008`** | TCP / SOCKS5 / HTTP | Secondary Multiplexer / Failover | Dipantau Aktif (Active Health Probe) |
| **`127.0.0.1:9090`** | TCP / REST API | Internal Diagnostic & State Exposer | Memory State Watchdog |

---

## 📊 Diagram Alir Sistem (Architecture Flow)

```text
+-----------------------------------------------------------------------+
|                       ANDROID OPERATING SYSTEM                        |
|  [BootReceiver]  --->  [IndogaroForegroundService]  ---> [WakeLock]   |
+-----------------------------------+-----------------------------------+
                                    | Spawns & Supervises
                                    v
+-----------------------------------------------------------------------+
|                    GO NATIVE SUPERVISOR ENGINE                        |
|  - RLIMIT_NOFILE (65535)             - Socket Health Watchdog         |
|  - GOMEMLIMIT (280MiB Pool)          - Autonomous In-Place Updater    |
|  - Zero I/O Stream (/dev/null)       - State Storage (state.json)     |
+-----------------------------------+-----------------------------------+
                                    | Spawns & Controls Sub-Binary
                                    v
+-----------------------------------------------------------------------+
|                    HIGH-PERFORMANCE PROXY CARRIER                     |
|                                                                       |
|   [Primary Carrier: 127.0.0.3:2007] <====> [v2rayNG / Clash Client]   |
|   [Failover Carrier: 127.0.0.3:2008] <===> [HTTP Custom / Nekobox]    |
+-----------------------------------------------------------------------+