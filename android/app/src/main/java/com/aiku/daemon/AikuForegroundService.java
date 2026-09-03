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
import android.util.Log;

import java.io.File;
import java.io.FileOutputStream;
import java.io.InputStream;
import java.io.OutputStream;

public class AikuForegroundService extends Service {
    private static final String TAG = "AikuService";
    private static final String CHANNEL_ID = "aiku_daemon_channel";
    private Process daemonProcess;
    private Thread supervisorThread;
    private volatile boolean isRunning = false;

    @Override
    public void onCreate() {
        super.onCreate();
        createNotificationChannel();
    }

    @Override
    public int onStartCommand(Intent intent, int flags, int startId) {
        Notification notification = buildNotification("Aiku Engine Service Running");
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

            Log.i(TAG, "Extracting full assets bundle (configs, blocklists, bin)...");
            extractAssetsRecursive("", filesDir);

            // Grant executable permissions
            binFile.setExecutable(true, false);
            File binDir = new File(filesDir, "bin");
            if (binDir.exists() && binDir.isDirectory()) {
                File[] subBins = binDir.listFiles();
                if (subBins != null) {
                    for (File f : subBins) {
                        f.setExecutable(true, false);
                    }
                }
            }

            while (isRunning) {
                try {
                    Log.i(TAG, "Spawning native Aiku engine...");
                    ProcessBuilder pb = new ProcessBuilder(binFile.getAbsolutePath());
                    pb.directory(filesDir);
                    pb.environment().put("ANDROID_DATA_DIR", filesDir.getAbsolutePath());
                    pb.redirectErrorStream(true);

                    daemonProcess = pb.start();
                    int exitCode = daemonProcess.waitFor();
                    Log.w(TAG, "Daemon stopped with code " + exitCode + ". Auto-restarting in 2s...");
                } catch (Exception e) {
                    Log.e(TAG, "Supervisor exception: " + e.getMessage(), e);
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
            Log.e(TAG, "Asset copy failed for path: " + assetSubDir + " : " + e.getMessage());
        }
    }

    private void copyFileAsset(String assetPath, File outFile) {
        if (outFile.exists() && outFile.length() > 0 && !assetPath.equals("aiku-daemon")) {
            return; // Skip if already extracted
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
            Log.e(TAG, "Error writing asset " + assetPath + ": " + e.getMessage());
        }
    }

    private void createNotificationChannel() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            NotificationChannel channel = new NotificationChannel(
                    CHANNEL_ID,
                    "Aiku Core Service",
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

        return builder.setContentTitle("Aiku Core Engine")
                .setContentText(text)
                .setSmallIcon(android.R.drawable.stat_notify_sync)
                .setContentIntent(pendingIntent)
                .build();
    }

    @Override
    public void onDestroy() {
        isRunning = false;
        if (daemonProcess != null) daemonProcess.destroy();
        super.onDestroy();
    }

    @Override
    public IBinder onBind(Intent intent) {
        return null;
    }
}
