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

import java.io.BufferedReader;
import java.io.File;
import java.io.FileOutputStream;
import java.io.InputStream;
import java.io.InputStreamReader;
import java.io.OutputStream;

public class AikuForegroundService extends Service {
    private static final String TAG = "AikuEngine";
    private static final String CHANNEL_ID = "aiku_network_channel";
    private Process daemonProcess;
    private Thread supervisorThread;
    private PowerManager.WakeLock wakeLock;
    private volatile boolean isRunning = false;

    @Override
    public void onCreate() {
        super.onCreate();
        createNotificationChannel();

        PowerManager pm = (PowerManager) getSystemService(POWER_SERVICE);
        if (pm != null) {
            wakeLock = pm.newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "AikuDaemon:WakeLock");
            wakeLock.acquire();
        }
    }

    @Override
    public int onStartCommand(Intent intent, int flags, int startId) {
        Notification notification = buildNotification("Aiku Core & Binary 'coba' Active");
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
            File binDir = new File(filesDir, "bin");
            binDir.mkdirs();

            Log.i(TAG, "Extracting full assets (aiku-daemon, coba, blocklists)...");
            extractAssetsRecursive("", filesDir);

            // Set Permission 755 & Executable
            enforcePermissions(filesDir);

            File daemonBin = new File(filesDir, "aiku-daemon");
            File cobaBin = new File(binDir, "coba");

            Log.i(TAG, "Target Daemon: " + daemonBin.getAbsolutePath() + " (Exists: " + daemonBin.exists() + ")");
            Log.i(TAG, "Target Binary coba: " + cobaBin.getAbsolutePath() + " (Exists: " + cobaBin.exists() + ")");

            while (isRunning) {
                try {
                    Log.i(TAG, "Spawning native Aiku Supervisor Daemon...");
                    ProcessBuilder pb = new ProcessBuilder(daemonBin.getAbsolutePath());
                    pb.directory(filesDir);

                    // Inject environment variables
                    pb.environment().put("ANDROID_DATA_DIR", filesDir.getAbsolutePath());
                    pb.environment().put("BIN_DIR", binDir.getAbsolutePath());
                    pb.environment().put("PATH", binDir.getAbsolutePath() + ":" + filesDir.getAbsolutePath() + ":" + System.getenv("PATH"));
                    pb.redirectErrorStream(true);

                    daemonProcess = pb.start();

                    // Baca log output langsung ke Logcat Android
                    BufferedReader reader = new BufferedReader(new InputStreamReader(daemonProcess.getInputStream()));
                    String line;
                    while ((line = reader.readLine()) != null) {
                        Log.i("AIKU_CORE", line);
                    }

                    int exitCode = daemonProcess.waitFor();
                    Log.w(TAG, "Core daemon stopped (" + exitCode + "). Auto-restarting in 2s...");
                } catch (Exception e) {
                    Log.e(TAG, "Failed executing daemon: " + e.getMessage(), e);
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

    private void enforcePermissions(File file) {
        if (!file.exists()) return;

        file.setReadable(true, false);
        file.setWritable(true, false);
        file.setExecutable(true, false);

        if (file.isDirectory()) {
            File[] children = file.listFiles();
            if (children != null) {
                for (File child : children) {
                    enforcePermissions(child);
                }
            }
        }

        try {
            Runtime.getRuntime().exec("chmod 755 " + file.getAbsolutePath()).waitFor();
        } catch (Exception ignored) {}
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
                if (!dir.exists()) dir.mkdirs();
                for (String asset : assets) {
                    if (asset.equals("images") || asset.equals("webkit")) continue;
                    String subPath = assetSubDir.isEmpty() ? asset : assetSubDir + "/" + asset;
                    extractAssetsRecursive(subPath, targetDir);
                }
            }
        } catch (Exception e) {
            Log.e(TAG, "Extraction error on " + assetSubDir + ": " + e.getMessage());
        }
    }

    private void copyFileAsset(String assetPath, File outFile) {
        outFile.getParentFile().mkdirs();
        try (InputStream in = getAssets().open(assetPath);
             OutputStream out = new FileOutputStream(outFile)) {
            byte[] buffer = new byte[16384];
            int read;
            while ((read = in.read(buffer)) != -1) {
                out.write(buffer, 0, read);
            }
            out.flush();
            outFile.setExecutable(true, false);
        } catch (Exception e) {
            Log.e(TAG, "Asset copy failed: " + assetPath + " -> " + e.getMessage());
        }
    }

    private void createNotificationChannel() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            NotificationChannel channel = new NotificationChannel(
                    CHANNEL_ID,
                    "Aiku Background Service",
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

        return builder.setContentTitle("Aiku Engine")
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
