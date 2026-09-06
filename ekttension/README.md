# 🧩 Indogaro Service Extensions (ekttension)

Kumpulan modul ekstensi, tools pendukung, dan sidecar controller untuk **Indogaro Core Service**.

---

## 📂 Struktur Direktori

| Direktori | Deskripsi | Target Platform |
|---|---|---|
| **`ekttension/cli/`** | Biner utilitas native Go untuk benchmark socket, probing latency, dan status probe `127.0.0.3:2007/2008`. | Android ARM64 / Linux |
| **`ekttension/scripts/`** | Kumpulan script shell untuk quick toggle ADB, memory flush, dan automated testing. | Android Shell / Termux / PC |
| **`ekttension/companion_apk/`** | Modul Android pendukung untuk Quick Settings Tile (Tombol di Status Bar) & Floating Widget. | Android 9 - 15 |

---

## ⚡ 1. Indogaro CLI Sidecar (`ekttension/cli`)

### Kompilasi Biner:
