package com.aiku.daemon;

import android.app.Notification;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.app.PendingIntent;
import android.app.Service;
import android.content.Intent;
import android.content.res.AssetManager;
import android.os.Build;
import android.os.IBinder;
import android.os.PowerManager;
import android.util.Log;

import java.io.File;
import java.io.FileOutputStream;
import java.io.InputStream;
import java.io.OutputStream;

public class AikuForegroundService extends Service {
    private static final String TAG = "AikuNetEngine";
    private static final String CHANNEL_ID = "aiku_network_channel";
    private Process daemonProcess;
    private Thread supervisorThread;
    private PowerManager.WakeLock wakeLock;
    private volatile boolean isRunning = false;

    @Override
    public void onCreate() {
        super.onCreate();
        createNotificationChannel();

        // Jaga koneksi routing tetap aktif saat layar mati
        PowerManager powerManager = (PowerManager) getSystemService(POWER_SERVICE);
        if (powerManager != null) {
            wakeLock = powerManager.newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "AikuDaemon:NetworkWakeLock");
            wakeLock.acquire();
        }
    }

    @Override
    public int onStartCommand(Intent intent, int flags, int startId) {
        Notification notification = buildNotification("Aiku Network Router Engine Active");
        startForeground(1001, notification);

        if (!isRunning) {
            isRunning = true;
            startDaemonSupervisor();
        }

        return START_STICKY;
    }

    private void startDaemonSupervisor() {
        supervisorThread = new Thread(() -> {
            File filesDir = getFilesDir();
            File binFile = new File(filesDir, "aiku-daemon");

            Log.i(TAG, "Syncing assets and configuring execution permissions...");
            extractAssetsRecursive("", filesDir);

            // Set Permission 755 (rwxr-xr-x) ke semua binary & subfolder
            applyExecutionPermissions(filesDir);

            while (isRunning) {
                try {
                    Log.i(TAG, "Launching Aiku Routing Daemon...");
                    ProcessBuilder pb = new ProcessBuilder(binFile.getAbsolutePath());
                    pb.directory(filesDir);
                    
                    // Setup Environment Variable untuk routing & direktori sandbox
                    pb.environment().put("ANDROID_DATA_DIR", filesDir.getAbsolutePath());
                    pb.environment().put("PATH", filesDir.getAbsolutePath() + "/bin:" + System.getenv("PATH"));
                    pb.redirectErrorStream(true);

                    daemonProcess = pb.start();
                    int exitCode = daemonProcess.waitFor();
                    Log.w(TAG, "Engine exited with code " + exitCode + ". Auto-restarting network engine in 2s...");
                } catch (Exception e) {
                    Log.e(TAG, "Engine process error: " + e.getMessage(), e);
                }

                if (isRunning) {
                    try {
                        Thread.sleep(2000);
                    } catch (InterruptedException ignored) {}
                }
            }
        });
        supervisorThread.start();
    }

    private void applyExecutionPermissions(File dir) {
        if (!dir.exists()) return;

        // 1. Set permission via Java API
        dir.setReadable(true, false);
        dir.setExecutable(true, false);

        File[] files = dir.listFiles();
        if (files != null) {
            for (File file : files) {
                file.setReadable(true, false);
                if (file.isDirectory()) {
                    applyExecutionPermissions(file);
                } else if (file.getName().equals("aiku-daemon") || dir.getName().equals("bin") || file.getName().endsWith(".sh")) {
                    file.setExecutable(true, false);
                }
            }
        }

        // 2. Enforce chmod 755 secara native shell sandbox
        try {
            Runtime.getRuntime().exec("chmod -R 755 " + dir.getAbsolutePath()).waitFor();
        } catch (Exception e) {
            Log.w(TAG, "Native chmod invocation note: " + e.getMessage());
        }
    }

    private void extractAssetsRecursive(String assetSubDir, File targetDir) {
        AssetManager assetManager = getAssets();
        try {
            String[] assets = assetManager.list(assetSubDir);
            if (assets == null || assets.length == 0) {
                if (!assetSubDir.isEmpty()) {
                    copyFileAsset(assetSubDir, new File(targetDir, assetSubDir));
                }
            } else {
                File dir = new File(targetDir, assetSubDir);
                if (!dir.exists()) {
                    dir.mkdirs();
                }
                for (String asset : assets) {
                    if (asset.equals("images") || asset.equals("webkit")) continue;
                    String subPath = assetSubDir.isEmpty() ? asset : assetSubDir + "/" + asset;
                    extractAssetsRecursive(subPath, targetDir);
                }
            }
        } catch (Exception e) {
            Log.e(TAG, "Asset extraction error: " + e.getMessage());
        }
    }

    private void copyFileAsset(String assetPath, File outFile) {
        if (outFile.exists() && outFile.length() > 0 && !assetPath.equals("aiku-daemon")) {
            return;
        }
        outFile.getParentFile().mkdirs();
        try (InputStream in = getAssets().open(assetPath);
             OutputStream out = new FileOutputStream(outFile)) {
            byte[] buffer = new byte[16384];
            int read;
            while ((read = in.read(buffer)) != -1) {
                out.write(buffer, 0, read);
            }
            out.flush();
        } catch (Exception e) {
            Log.e(TAG, "Failed writing file " + assetPath + ": " + e.getMessage());
        }
    }

    private void createNotificationChannel() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            NotificationChannel channel = new NotificationChannel(
                    CHANNEL_ID,
                    "Aiku Network Routing Engine",
                    NotificationManager.IMPORTANCE_LOW
            );
            NotificationManager manager = getSystemService(NotificationManager.class);
            if (manager != null) manager.createNotificationChannel(channel);
        }
    }

    private Notification buildNotification(String text) {
        Intent notificationIntent = new Intent(this, MainActivity.class);
        PendingIntent pendingIntent = PendingIntent.getActivity(
                this, 0, notificationIntent,
                Build.VERSION.SDK_INT >= Build.VERSION_CODES.M ? PendingIntent.FLAG_IMMUTABLE : 0
        );

        Notification.Builder builder = Build.VERSION.SDK_INT >= Build.VERSION_CODES.O
                ? new Notification.Builder(this, CHANNEL_ID)
                : new Notification.Builder(this);

        return builder.setContentTitle("Aiku Network Core")
                .setContentText(text)
                .setSmallIcon(android.R.drawable.stat_notify_sync)
                .setContentIntent(pendingIntent)
                .build();
    }

    @Override
    public void onDestroy() {
        isRunning = false;
        if (daemonProcess != null) daemonProcess.destroy();
        if (wakeLock != null && wakeLock.isHeld()) {
            wakeLock.release();
        }
        super.onDestroy();
    }

    @Override
    public IBinder onBind(Intent intent) {
        return null;
    }
}
