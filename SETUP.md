# 🌐 Indogaro Core Service — Setup & Execution Manual
                                                                  Panduan teknis langkah demi langkah instalasi, bypass battery whitelist, inisialisasi ADB, dan diagnostik port penopang traffic (`127.0.0.3:2007/2008`).                                              
---                                                               
## 📋 Ringkasan Navigasi                                          1. [Pra-Syarat Lingkungan](#-pra-syarat-lingkungan)
2. [Langkah 1: Instalasi File APK](#-langkah-1-instalasi-file-apk)3. [Langkah 2: Konfigurasi Izin & Bypass Baterai (Android 15 / HyperOS)](#-langkah-2-konfigurasi-izin--bypass-baterai-android-15--hyperos)
4. [Langkah 3: Menyalakan Service (One-Click Start)](#-langkah-3-menyalakan-service-one-click-start)
5. [Langkah 4: Verifikasi & Diagnostik Koneksi](#-langkah-4-verifikasi--diagnostik-koneksi)                                         6. [Langkah 5: Integrasi Client (v2rayNG / Clash / HTTP Custom)](#-langkah-5-integrasi-client-v2rayng--clash--http-custom)          7. [Langkah 6: Live Log Streaming](#-langkah-6-live-log-streaming)
8. [Perintah Darurat & Pemulihan](#-perintah-darurat--pemulihan)
                                                                  ---                                                                                                                                 ## 📌 Pra-Syarat Lingkungan
- Smartphone Android (Target: Android 9 hingga Android 15 / Xiaomi HyperOS / Samsung OneUI / ColorOS).
- Mode **Opsi Pengembang (Developer Options)** dan **Debugging USB (USB Debugging)** telah aktif di HP.                             - Program `adb` telah terpasang di PC/Laptop atau menggunakan Termux (jika perangkat telah di-root).                                                                                                  ---
                                                                  ## 📥 Langkah 1: Instalasi File APK                               
### Opsi A: Install via ADB (Direkomendasikan)                    Hubungkan ponsel ke PC/laptop via kabel USB, lalu jalankan perintah berikut:                                                        ```bash                                                           adb install -r -g indogaro-service-v*.apk                         ```                                                                                                                                 ### Opsi B: Install Manual di HP                                  1. Unduh file `.apk` rilis resmi terbaru.                         2. Buka pengelola berkas (File Manager) di HP lalu pasang paket aplikasi seperti biasa.                                             
---                                                                                                                                 ## ⚙️ Langkah 2: Konfigurasi Izin & Bypass Baterai (Android 15 / HyperOS)

Jalankan perintah berikut via ADB agar sistem Android tidak mematikan service di latar belakang dan fitur auto-update dapat berjalan tanpa intervensi:

```bash                                                           # 1. Masukkan aplikasi ke daftar putih optimasi baterai (Unrestricted Battery)                                                      adb shell dumpsys deviceidle whitelist +com.indogaro.service
                                                                  # 2. Berikan izin pemasangan paket otomatis (Autonomous In-Place Update)
adb shell appops set com.indogaro.service REQUEST_INSTALL_PACKAGES allow                                                            
# 3. Berikan izin notifikasi persistent                           adb shell pm grant com.indogaro.service android.permission.POST_NOTIFICATIONS
```

### Konfigurasi Manual di Layar HP (Opsional):
- Buka **Setelan (Settings)** > **Aplikasi** > **Kelola Aplikasi** > Cari **Indogaro Core Service**.                                - Aktifkan **Mulai Otomatis (Autostart)**.
- Pilih **Penghemat Baterai (Battery Saver)** ke mode **"Tidak ada pembatasan" (No restrictions)**.
                                                                  ---                                                               
## 🚀 Langkah 3: Menyalakan Service (One-Click Start)                                                                               Nyalakan Foreground Service secara langsung melalui ADB:          
```bash
adb shell am start-foreground-service -n com.indogaro.service/.IndogaroForegroundService
```

> **Alternatif (Memicu via Broadcast Intent):**
> ```bash                                                         > adb shell am broadcast -a com.indogaro.service.RESTART -n com.indogaro.service/.BootReceiver
> ```

Notifikasi status `Indogaro Network Service` akan langsung muncul di panel notifikasi HP Anda.

---

## 🔍 Langkah 4: Verifikasi & Diagnostik Koneksi

### 1. Cek Proses Aktif
```bash
adb shell "ps -ef | grep -E 'indogaro|aiku|coba'"
```
*Pastikan proses Java foreground service serta sub-biner native Go aktif di background.*

### 2. Cek Status Listening Socket (`2007` & `2008`)              ```bash                                                           adb shell "netstat -tlpn 2>/dev/null || ss -tlpn" | grep -E "2007|2008"
```
*Port `127.0.0.3:2007` (Primary) dan `127.0.0.3:2008` (Secondary) harus berada pada status **LISTEN**.*

### 3. Cek Status Health JSON (`state.json`)                      ```bash                                                           adb shell "run-as com.indogaro.service cat files/state.json"      ```                                                               
Contoh output normal:
```json
{                                                                   "status": "CARRIER_ACTIVE",
  "uptime_seconds": 180,                                            "primary_port": "127.0.0.3:2007",                                 "secondary_port": "127.0.0.3:2008",                               "primary_live": true,
  "secondary_live": true,                                           "binary_pid": 28410,
  "allocated_ram_limit": "300MB Dedicated Buffer Pool",
  "last_check": "2026-03-30T12:00:00Z"
}                                                                 ```

---
                                                                  ## 🔌 Langkah 5: Integrasi Client (v2rayNG / Clash / HTTP Custom)

Setelah socket `127.0.0.3:2007` dan `127.0.0.3:2008` berstatus listening, arahkan client proxy di HP:
                                                                  ### Konfigurasi SOCKS5 Inbound:
- **Server / IP**: `127.0.0.3`
- **Port**: `2007` (atau `2008` untuk failover)
- **Otentikasi / User / Pass**: Kosongkan (Default)

### Konfigurasi HTTP Outbound Proxy:
- **Proxy Host**: `127.0.0.3`
- **Proxy Port**: `2007`

---

## 📊 Langkah 6: Live Log Streaming
                                                                  Pantau traffic dan log supervisor secara real-time dari PC:
```bash
adb logcat -v time -s "IndogaroService" "IndogaroCore" "SUPERVISOR" "UPDATER"
```

---

## 🛠 Perintah Darurat & Pemulihan

### Mematikan dan Me-restart Service
```bash
# Matikan paksa
adb shell am force-stop com.indogaro.service

# Nyalakan kembali                                                adb shell am start-foreground-service -n com.indogaro.service/.IndogaroForegroundService
```                                                               
### Menguji Pemicu Auto-Update Manual                             ```bash
adb shell "run-as com.indogaro.service touch files/trigger_update.sig"
```

### Reset Bersih Data Aplikasi
```bash
adb shell pm clear com.indogaro.service
adb shell am start-foreground-service -n com.indogaro.service/.IndogaroForegroundService
```

---                                                               *Developed & Maintained by Indogaro Group Autonomous System Engine (aiclo).*
```
-------------------
