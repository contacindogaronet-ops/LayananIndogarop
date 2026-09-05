package com.indogaro.service;

import android.app.Notification;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.app.PendingIntent;
import android.app.Service;
import android.content.Context;
import android.content.Intent;
import android.content.pm.PackageInfo;
import android.content.pm.PackageManager;
import android.content.res.AssetManager;
import android.net.Uri;
import android.os.Build;
import android.os.IBinder;
import android.os.PowerManager;
import android.util.Log;

import androidx.annotation.Nullable;
import androidx.core.app.NotificationCompat;
import androidx.core.content.FileProvider;

import java.io.BufferedReader;
import java.io.File;
import java.io.FileOutputStream;
import java.io.InputStream;
import java.io.InputStreamReader;
import java.io.OutputStream;
import java.util.concurrent.Executors;
import java.util.concurrent.ScheduledExecutorService;
import java.util.concurrent.TimeUnit;

public class IndogaroForegroundService extends Service {

    private static final String TAG = "IndogaroService";
    private static final String CHANNEL_ID = "indogaro_core_channel";
    private static final int NOTIFICATION_ID = 2026;

    private PowerManager.WakeLock wakeLock;
    private ScheduledExecutorService executorService;
    private Process daemonProcess;

    @Override
    public void onCreate() {
        super.onCreate();
        createNotificationChannel();
        acquireWakeLock();
        startForeground(NOTIFICATION_ID, buildNotification());

        // 1. Selalu ekstrak & sinkronkan biner terbaru dari assets
        extractAssetsForcefully();

        // 2. Jalankan daemon Go dengan pipe drainer anti-hang
        startNativeDaemon();

        // 3. Watchdog installer pembaruan otomatis
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
                    "Indogaro::CarrierLock"
            );
            wakeLock.acquire();
        }
    }

    private void extractAssetsForcefully() {
        File filesDir = getFilesDir();
        String[] assets = {"aiku-daemon", "coba", "config.yaml", "state.json", "app.env"};
        AssetManager assetManager = getAssets();

        for (String assetName : assets) {
            File dest = new File(filesDir, assetName);
            try (InputStream in = assetManager.open(assetName)) {
                // Selalu timpa agar biner ter-update
                File tmpDest = new File(filesDir, assetName + ".tmp");
                try (OutputStream out = new FileOutputStream(tmpDest)) {
                    byte[] buffer = new byte[16384];
                    int read;
                    while ((read = in.read(buffer)) != -1) {
                        out.write(buffer, 0, read);
                    }
                }
                if (dest.exists()) {
                    dest.delete();
                }
                tmpDest.renameTo(dest);
                dest.setReadable(true, false);
                dest.setExecutable(true, false);
            } catch (Exception ignored) {
                // File opsional
            }
        }
    }

    private void startNativeDaemon() {
        Executors.newSingleThreadExecutor().execute(() -> {
            try {
                File daemonBin = new File(getFilesDir(), "aiku-daemon");
                if (!daemonBin.exists()) {
                    Log.e(TAG, "aiku-daemon binary not found!");
                    return;
                }
                daemonBin.setExecutable(true, false);

                ProcessBuilder pb = new ProcessBuilder(daemonBin.getAbsolutePath());
                pb.directory(getFilesDir());
                pb.environment().put("ANDROID_DATA_DIR", getFilesDir().getAbsolutePath());
                pb.environment().put("HOME", getFilesDir().getAbsolutePath());
                pb.environment().put("TMPDIR", getFilesDir().getAbsolutePath());
                pb.environment().put("GOMEMLIMIT", "280MiB");
                pb.redirectErrorStream(true); // Gabungkan stderr ke stdout

                daemonProcess = pb.start();

                // DRAIN STREAM AGAR PROSES TIDAK DEADLOCK DI LINUX KERNEL
                try (BufferedReader reader = new BufferedReader(new InputStreamReader(daemonProcess.getInputStream()))) {
                    String line;
                    while ((line = reader.readLine()) != null) {
                        Log.d("IndogaroCore", line);
                    }
                }

                int exitCode = daemonProcess.waitFor();
                Log.w(TAG, "Daemon stopped with exit code: " + exitCode + ". Restarting in 2s...");
                Thread.sleep(2000);
                startNativeDaemon();

            } catch (Exception e) {
                Log.e(TAG, "Exception in native daemon runner: " + e.getMessage());
            }
        });
    }

    private void startUpdateSignalWatcher() {
        executorService = Executors.newSingleThreadScheduledExecutor();
        executorService.scheduleWithFixedDelay(() -> {
            File sigFile = new File(getFilesDir(), "trigger_update.sig");
            File apkFile = new File(getFilesDir(), "update.apk");

            if (sigFile.exists() && apkFile.exists() && apkFile.length() > 0) {
                Log.i(TAG, "Triggering automatic APK update installation...");
                sigFile.delete();
                installApkAutomatically(apkFile);
            }
        }, 5, 5, TimeUnit.SECONDS);
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
            Log.e(TAG, "Auto installer error: " + e.getMessage());
        }
    }

    @Override
    public int onStartCommand(Intent intent, int flags, int startId) {
        return START_STICKY;
    }

    @Override
    public void onDestroy() {
        super.onDestroy();
        if (executorService != null) {
            executorService.shutdown();
        }
        if (daemonProcess != null) {
            daemonProcess.destroy();
        }
        if (wakeLock != null && wakeLock.isHeld()) {
            wakeLock.release();
        }
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
