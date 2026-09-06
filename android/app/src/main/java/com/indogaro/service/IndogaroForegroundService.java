package com.indogaro.service;

import android.app.Notification;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.app.PendingIntent;
import android.app.Service;
import android.content.Context;
import android.content.Intent;
import android.net.Uri;
import android.os.Build;
import android.os.FileObserver;
import android.os.IBinder;
import android.os.PowerManager;
import android.util.Log;
import androidx.core.app.NotificationCompat;
import androidx.core.content.FileProvider;
import java.io.File;
import java.io.FileOutputStream;
import java.io.InputStream;

public class IndogaroForegroundService extends Service {
    private static final String TAG = "IndogaroService";
    private static final String CHANNEL_ID = "indogaro_service_channel";
    private static final String UPDATE_CHANNEL_ID = "indogaro_update_channel";
    private static final int NOTIFICATION_ID = 1001;
    public static final int UPDATE_NOTIF_ID = 1002;

    private PowerManager.WakeLock wakeLock;
    private Process nativeProcess;
    private FileObserver updateObserver;

    @Override
    public void onCreate() {
        super.onCreate();
        createNotificationChannels();
        acquireWakeLock();
        startForeground(NOTIFICATION_ID, buildForegroundNotification("Memulai Indogaro Core Service..."));
        
        setupUpdateObserver();
        launchNativeDaemon();
    }

    private void createNotificationChannels() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            NotificationManager manager = (NotificationManager) getSystemService(Context.NOTIFICATION_SERVICE);
            if (manager == null) return;

            // Channel status service
            NotificationChannel serviceChannel = new NotificationChannel(
                    CHANNEL_ID,
                    "Indogaro Service Runtime",
                    NotificationManager.IMPORTANCE_LOW
            );
            serviceChannel.setDescription("Menjaga koneksi proxy dan supervisor carrier aktif di background.");
            manager.createNotificationChannel(serviceChannel);

            // Channel notifikasi update interaktif
            NotificationChannel updateChannel = new NotificationChannel(
                    UPDATE_CHANNEL_ID,
                    "Indogaro System Updates",
                    NotificationManager.IMPORTANCE_HIGH
            );
            updateChannel.setDescription("Notifikasi ketersediaan update biner dan paket Indogaro Core.");
            manager.createNotificationChannel(updateChannel);
        }
    }

    private void acquireWakeLock() {
        PowerManager powerManager = (PowerManager) getSystemService(Context.POWER_SERVICE);
        if (powerManager != null) {
            wakeLock = powerManager.newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "Indogaro::ServiceWakeLock");
            wakeLock.acquire();
        }
    }

    private Notification buildForegroundNotification(String contentText) {
        return new NotificationCompat.Builder(this, CHANNEL_ID)
                .setContentTitle("Indogaro Core Service")
                .setContentText(contentText)
                .setSmallIcon(android.R.drawable.stat_notify_sync)
                .setPriority(NotificationCompat.PRIORITY_LOW)
                .setOngoing(true)
                .build();
    }

    public void showUpdateAvailableNotification(String apkPath, String versionName) {
        File apkFile = new File(apkPath);
        if (!apkFile.exists()) return;

        Intent updateIntent = new Intent(this, UpdateReceiver.class);
        updateIntent.setAction(UpdateReceiver.ACTION_INSTALL_UPDATE);
        updateIntent.putExtra(UpdateReceiver.EXTRA_APK_PATH, apkPath);

        int flags = PendingIntent.FLAG_UPDATE_CURRENT;
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.M) {
            flags |= PendingIntent.FLAG_IMMUTABLE;
        }
        PendingIntent pendingIntent = PendingIntent.getBroadcast(this, 0, updateIntent, flags);

        NotificationCompat.Builder builder = new NotificationCompat.Builder(this, UPDATE_CHANNEL_ID)
                .setContentTitle("⚡ Pembaruan Tersedia " + (versionName != null ? versionName : ""))
                .setContentText("Versi baru siap diinstal. Sentuh atau klik update sekarang.")
                .setSmallIcon(android.R.drawable.stat_sys_download_done)
                .setPriority(NotificationCompat.PRIORITY_HIGH)
                .setAutoCancel(true)
                .setContentIntent(pendingIntent)
                .addAction(android.R.drawable.ic_menu_upload, "UPDATE SEKARANG", pendingIntent);

        NotificationManager manager = (NotificationManager) getSystemService(Context.NOTIFICATION_SERVICE);
        if (manager != null) {
            manager.notify(UPDATE_NOTIF_ID, builder.build());
        }
    }

    private void setupUpdateObserver() {
        File updateDir = new File(getFilesDir(), "updates");
        if (!updateDir.exists()) {
            updateDir.mkdirs();
        }

        updateObserver = new FileObserver(updateDir.getAbsolutePath(), FileObserver.CLOSE_WRITE | FileObserver.MOVED_TO) {
            @Override
            public void onEvent(int event, String path) {
                if (path != null && path.endsWith(".apk")) {
                    File newApk = new File(updateDir, path);
                    Log.i(TAG, "File APK update terdeteksi selesai diunduh: " + newApk.getAbsolutePath());
                    showUpdateAvailableNotification(newApk.getAbsolutePath(), path.replace(".apk", ""));
                }
            }
        };
        updateObserver.startWatching();
    }

    private void launchNativeDaemon() {
        new Thread(() -> {
            try {
                File binDir = new File(getFilesDir(), "bin");
                if (!binDir.exists()) binDir.mkdirs();

                File binary = new File(binDir, "coba");
                if (!binary.exists() || binary.length() == 0) {
                    extractAssetBinary("coba", binary);
                }
                binary.setExecutable(true, false);

                ProcessBuilder pb = new ProcessBuilder(binary.getAbsolutePath());
                pb.directory(getFilesDir());
                pb.environment().put("GOMEMLIMIT", "280MiB");
                pb.environment().put("HOME", getFilesDir().getAbsolutePath());
                
                // Zero I/O silence
                File devNull = new File("/dev/null");
                pb.redirectOutput(ProcessBuilder.Redirect.to(devNull));
                pb.redirectError(ProcessBuilder.Redirect.to(devNull));

                nativeProcess = pb.start();
                Log.i(TAG, "Native core engine berhasil dieksekusi.");

                NotificationManager manager = (NotificationManager) getSystemService(Context.NOTIFICATION_SERVICE);
                if (manager != null) {
                    manager.notify(NOTIFICATION_ID, buildForegroundNotification("Carrier Aktif: 127.0.0.3:2007/2008"));
                }

                nativeProcess.waitFor();
            } catch (Exception e) {
                Log.e(TAG, "Error saat menjalankan native daemon: " + e.getMessage(), e);
            }
        }).start();
    }

    private void extractAssetBinary(String assetName, File destination) {
        try (InputStream in = getAssets().open(assetName);
             FileOutputStream out = new FileOutputStream(destination)) {
            byte[] buffer = new byte[8192];
            int read;
            while ((read = in.read(buffer)) != -1) {
                out.write(buffer, 0, read);
            }
            out.flush();
        } catch (Exception e) {
            Log.e(TAG, "Gagal mengekstrak binary asset: " + e.getMessage(), e);
        }
    }

    @Override
    public int onStartCommand(Intent intent, int flags, int startId) {
        return START_STICKY;
    }

    @Override
    public void onDestroy() {
        super.onDestroy();
        if (updateObserver != null) {
            updateObserver.stopWatching();
        }
        if (nativeProcess != null) {
            nativeProcess.destroy();
        }
        if (wakeLock != null && wakeLock.isHeld()) {
            wakeLock.release();
        }
    }

    @Override
    public IBinder onBind(Intent intent) {
        return null;
    }
}