#!/bin/bash
# Indogaro Service Controller & Quick Tool
# Developed by aiclo (Indogaro Group)

PACKAGE="com.indogaro.service"
SERVICE_NAME="com.indogaro.service/.IndogaroForegroundService"
RECEIVER_NAME="com.indogaro.service/.BootReceiver"

print_header() {
    echo "================================================="
    echo "       ⚡ INDOGARO CORE EXTENSION CONTROLLER ⚡  "
    echo "================================================="
}

case "$1" in
    start)
        print_header
        echo "[+] Menyalakan Indogaro Foreground Service..."
        adb shell am start-foreground-service -n "$SERVICE_NAME"
        ;;
    stop)
        print_header
        echo "[+] Menghentikan Service..."
        adb shell am force-stop "$PACKAGE"
        ;;
    restart)
        print_header
        echo "[+] Me-restart Service..."
        adb shell am force-stop "$PACKAGE"
        sleep 1
        adb shell am start-foreground-service -n "$SERVICE_NAME"
        ;;
    status)
        print_header
        echo "[+] Mengecek Status Proses:"
        adb shell "ps -ef | grep -E 'indogaro|aiku|coba'"
        echo ""
        echo "[+] Mengecek Listening Socket (2007 / 2008):"
        adb shell "netstat -tlpn 2>/dev/null || ss -tlpn" | grep -E "2007|2008"
        ;;
    grant)
        print_header
        echo "[+] Memberikan seluruh izin background & bypass baterai..."
        adb shell dumpsys deviceidle whitelist +"$PACKAGE"
        adb shell appops set "$PACKAGE" REQUEST_INSTALL_PACKAGES allow
        adb shell pm grant "$PACKAGE" android.permission.POST_NOTIFICATIONS
        echo "[✓] Izin berhasil diberikan 100%."
        ;;
    logs)
        print_header
        echo "[+] Membuka streaming logcat realtime..."
        adb logcat -v time -s "IndogaroService" "IndogaroCore" "SUPERVISOR" "UPDATER"
        ;;
    *)
        print_header
        echo "Penggunaan: ./controller.sh {start|stop|restart|status|grant|logs}"
        ;;
esac
