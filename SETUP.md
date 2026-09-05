# 🌐 Indogaro Core Service — Setup & Deployment Guide

Panduan resmi instalasi, konfigurasi izin (HyperOS / Android 15), inisialisasi via ADB, dan verifikasi koneksi penopang traffic (300MB RAM & Dual Socket `127.0.0.3:2007/2008`).

---

## 📋 Daftar Isi
1. [Spesifikasi & Arsitektur](#-spesifikasi--arsitektur)
2. [Langkah 1: Instalasi APK](#-langkah-1-instalasi-apk)
3. [Langkah 2: Konfigurasi Izin Khusus (Android 15 / HyperOS)](#-langkah-2-konfigurasi-izin-khusus-android-15--hyperos)
4. [Langkah 3: Menyalakan Service via ADB (One-Click Initialization)](#-langkah-3-menyalakan-service-via-adb-one-click-initialization)
5. [Langkah 4: Verifikasi Status & Live Ports](#-langkah-4-verifikasi-status--live-ports)
6. [Langkah 5: Monitoring Live Logcat & Real-time Metrics](#-langkah-5-monitoring-live-logcat--real-time-metrics)
7. [Troubleshooting & Perintah Darurat](#-troubleshooting--perintah-darurat)

---

## 🏗 Spesifikasi & Arsitektur

| Komponen | Spesifikasi |
|---|---|
| **Package Name** | `com.indogaro.service` |
| **Foreground Service** | `com.indogaro.service/.IndogaroForegroundService` |
| **Broadcast Receiver** | `com.indogaro.service/.BootReceiver` |
| **Local Binding Socket** | `127.0.0.3:2007` & `127.0.0.3:2008` |
| **Memory Allocation (Heap + Daemon)** | LargeHeap Active + `GOMEMLIMIT=280MiB` (~300MB Pool) |
| **Auto-Updater Engine** | Autonomous In-Place Downloader via FileProvider |

---

## 📥 Langkah 1: Instalasi APK

### Opsi A: Install via ADB (Direkomendasikan)
Hubungkan HP ke PC dengan USB Debugging aktif, lalu jalankan:
