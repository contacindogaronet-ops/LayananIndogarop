package com.indogaro.service;

import android.app.Notification;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.app.PendingIntent;
import android.app.Service;
import android.content.Context;
import android.content.Intent;
import android.content.pm.PackageInstaller;
import android.content.res.AssetManager;
import android.net.Uri;
import android.os.Build;
import android.os.IBinder;
import android.os.PowerManager;
import android.util.Log;

import androidx.annotation.Nullable;
import androidx.core.app.NotificationCompat;
import androidx.core.content.FileProvider;

import java.io.File;
import java.io.FileInputStream;
import java.io.FileOutputStream;
import java.io.InputStream;
import java.io.OutputStream;
import java.util.concurrent.Executors;
import java.util.concurrent.ScheduledExecutorService;
import java.util.concurrent.TimeUnit;

public class IndogaroForegroundService extends Service {

    private static final String TAG = "IndogaroService";
    private static final String CHANNEL_ID = "indogaro_core_channel";
    private static final int NOTIFICATION_ID = 2026;

    private PowerManager.WakeLock wakeLock;
    private ScheduledExecutorService updateWatcherExecutor;
    private Process daemonProcess;

    @Override
    public void onCreate() {
        super.onCreate();
        createNotificationChannel();
        acquireWakeLock();
        startForeground(NOTIFICATION_ID, buildNotification());
        
        // Ekstrak aset biner awal jika belum ada
        extractAssetsIfNecessary();

        // Jalankan Native Go Daemon
        startNativeDaemon();

        // Mulai pemantauan otomatis file sinyal update
        startUpdateSignalWatcher();
    }

    private void createNotificationChannel() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            NotificationChannel channel = new NotificationChannel(
                    CHANNEL_ID,
                    getString(R.string.service_channel_name),
                    NotificationManager.IMPORTANCE_LOW
            );
            channel.setDescription(getString(R.string.service_channel_desc));
            channel.setShowBadge(false);

            NotificationManager manager = getSystemService(NotificationManager.class);
            if (manager != null) {
                manager.createNotificationChannel(channel);
            }
        }
    }

    private Notification buildNotification() {
        return new NotificationCompat.Builder(this, CHANNEL_ID)
                .setContentTitle(getString(R.string.notification_title))
                .setContentText(getString(R.string.notification_active))
                .setSmallIcon(android.R.drawable.stat_notify_sync)
                .setPriority(NotificationCompat.PRIORITY_LOW)
                .setOngoing(true)
                .build();
    }

    private void acquireWakeLock() {
        PowerManager powerManager = (PowerManager) getSystemService(Context.POWER_SERVICE);
        if (powerManager != null) {
            wakeLock = powerManager.newWakeLock(
                    PowerManager.PARTIAL_WAKE_LOCK,
                    "Indogaro::PersistentCarrierWakeLock"
            );
            wakeLock.acquire();
        }
    }

    private void extractAssetsIfNecessary() {
        File filesDir = getFilesDir();
        String[] binaries = {"aiku-daemon", "coba", "config.yaml", "state.json"};
        AssetManager assetManager = getAssets();

        for (String bin : binaries) {
            File dest = new File(filesDir, bin);
            if (!dest.exists()) {
                try (InputStream in = assetManager.open(bin);
                     OutputStream out = new FileOutputStream(dest)) {
                    byte[] buffer = new byte[8192];
                    int read;
                    while ((read = in.read(buffer)) != -1) {
                        out.write(buffer, 0, read);
                    }
                    dest.setExecutable(true, false);
                    dest.setReadable(true, false);
                } catch (Exception ignored) {
                }
            }
        }
    }

    private void startNativeDaemon() {
        Executors.newSingleThreadExecutor().execute(() -> {
            try {
                File daemonBin = new File(getFilesDir(), "aiku-daemon");
                if (daemonBin.exists()) {
                    daemonBin.setExecutable(true, false);
                    ProcessBuilder pb = new ProcessBuilder(daemonBin.getAbsolutePath());
                    pb.directory(getFilesDir());
                    pb.environment().put("ANDROID_DATA_DIR", getFilesDir().getAbsolutePath());
                    pb.environment().put("HOME", getFilesDir().getAbsolutePath());
                    pb.environment().put("TMPDIR", getFilesDir().getAbsolutePath());
                    pb.environment().put("GOMEMLIMIT", "280MiB");
                    daemonProcess = pb.start();
                }
            } catch (Exception e) {
                Log.e(TAG, "Gagal memulai daemon: " + e.getMessage());
            }
        });
    }

    private void startUpdateSignalWatcher() {
        updateWatcherExecutor = Executors.newSingleThreadScheduledExecutor();
        updateWatcherExecutor.scheduleWithFixedDelay(() -> {
            File sigFile = new File(getFilesDir(), "trigger_update.sig");
            File apkFile = new File(getFilesDir(), "update.apk");

            if (sigFile.exists() && apkFile.exists() && apkFile.length() > 0) {
                Log.i(TAG, "Memicu instalasi pembaruan APK otomatis...");
                sigFile.delete(); // Hapus trigger agar tidak dieksekusi berulang
                installApkAutomatically(apkFile);
            }
        }, 10, 10, TimeUnit.SECONDS);
    }

    private void installApkAutomatically(File apkFile) {
        try {
            Uri apkUri = FileProvider.getUriForFile(
                    this,
                    getPackageName() + ".fileprovider",
                    apkFile
            );

            Intent installIntent = new Intent(Intent.ACTION_VIEW);
            installIntent.setDataAndType(apkUri, "application/vnd.android.package-archive");
            installIntent.addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION);
            installIntent.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK);

            startActivity(installIntent);
        } catch (Exception e) {
            Log.e(TAG, "Gagal memulai auto installer: " + e.getMessage());
        }
    }

    @Override
    public int onStartCommand(Intent intent, int flags, int startId) {
        return START_STICKY;
    }

    @Override
    public void onDestroy() {
        super.onDestroy();
        if (updateWatcherExecutor != null) {
            updateWatcherExecutor.shutdown();
        }
        if (daemonProcess != null) {
            daemonProcess.destroy();
        }
        if (wakeLock != null && wakeLock.isHeld()) {
            wakeLock.release();
        }
        // Auto respawn jika service dihentikan OS
        Intent restartIntent = new Intent(getApplicationContext(), BootReceiver.class);
        restartIntent.setAction("com.indogaro.service.RESTART");
        sendBroadcast(restartIntent);
    }

    @Nullable
    @Override
    public IBinder onBind(Intent intent) {
        return null;
    }
}
