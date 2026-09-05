# 🌐 Indogaro Core Service — Setup & Execution Manual

Panduan teknis langkah demi langkah instalasi, bypass battery whitelist, inisialisasi ADB, dan diagnostik port penopang traffic (`127.0.0.3:2007/2008`).

---

## 📋 Ringkasan Navigasi
1. [Pra-Syarat Lingkungan](#-pra-syarat-lingkungan)
2. [Langkah 1: Instalasi File APK](#-langkah-1-instalasi-file-apk)
3. [Langkah 2: Konfigurasi Izin & Bypass Baterai (Android 15 / HyperOS)](#-langkah-2-konfigurasi-izin--bypass-baterai-android-15--hyperos)
4. [Langkah 3: Menyalakan Service (One-Click Start)](#-langkah-3-menyalakan-service-one-click-start)
5. [Langkah 4: Verifikasi & Diagnostik Koneksi](#-langkah-4-verifikasi--diagnostik-koneksi)
6. [Langkah 5: Live Log Streaming](#-langkah-5-live-log-streaming)
7. [Perintah Darurat & Pemulihan](#-perintah-darurat--pemulihan)

---

## 📌 Pra-Syarat Lingkungan
- Smartphone Android (Target: Android 9 hingga Android 15 / Xiaomi HyperOS / Samsung OneUI / ColorOS).
- Mode **Opsi Pengembang (Developer Options)** dan **Debugging USB (USB Debugging)** telah aktif di HP.
- Program `adb` telah terpasang di PC/Laptop.

---

## 📥 Langkah 1: Instalasi File APK

Pasang file APK langsung ke HP via terminal:
