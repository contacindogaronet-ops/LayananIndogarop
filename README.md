<div align="center">

# INDOGARO CORE SERVICE

**High-Performance Headless Network Daemon & Multiplexer Engine for Android**

[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](https://www.gnu.org/licenses/gpl-3.0)
[![Go Report Card](https://goreportcard.com/badge/github.com/indogaro/service)](https://goreportcard.com/report/github.com/indogaro/service)
[![Android Target](https://img.shields.io/badge/Android%20Target-SDK%2028%20(Pure%20Native)-brightgreen.svg)]()
[![Build & Release](https://github.com/indogaro/service/actions/workflows/build.yml/badge.svg)](https://github.com/indogaro/service/actions)

</div>

---

## ⚡ Ikhtisar Arsitektur

**Indogaro Core Service** (`com.indogaro.service`) adalah engine background headless tingkat lanjut yang menggabungkan kemampuan **Go Native Supervisor** dan **Android Low-Level Foreground Architecture** untuk menghasilkan koneksi proxy ultra-stabil dengan latensi dan jitter minimal.

### Keunggulan Arsitektur:
* **Pure Headless Design**: Tidak memuat antarmuka GUI/Activity berat; bebas dari memory leak WebView.
* **Zero-I/O Silence Mode (`/dev/null`)**: Memangkas penggunaan CPU dan disk I/O dengan mengisolasi log binary sub-layer.
* **TargetSdk 28 Standalone**: Bypass restriksi eksekusi biner SELinux dan isolasi loopback Android modern.
* **TCP Socket Deadlock Watchdog**: Memantau port `127.0.0.3:2007` & `2008` secara real-time dan memulihkan koneksi secara otomatis jika terjadi hang.
* **Persistent WakeLock & Anti-Kill**: Mencegah proses di-freeze oleh Doze Mode pada custom ROM (MIUI/HyperOS, OneUI, ColorOS).
* **Continuous In-Place Update**: Integrasi GitHub Release Updater dengan verifikasi integritas paket dan konsistensi keystore signing.

---

## 📥 Panduan Instalasi & Aktivasi

### 1. Unduh APK
Unduh APK rilis terbaru dari tab **[Releases](../../releases)**.

### 2. Pasang APK
Pasang APK ke perangkat Android via ADB atau file manager:
