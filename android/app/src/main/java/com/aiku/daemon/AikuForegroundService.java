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
    private static final String TAG = "AikuCore";
    private static final String CHANNEL_ID = "aiku_service_channel";
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
        Notification notification = buildNotification("Aiku Core Active | Routing 127.0.0.3:2007");
        startForeground(1001, notification);

        if (!isRunning) {
            isRunning = true;
            startDaemonSupervisor();
        }

        return START_STICKY;
    }

    private void startDaemonSupervisor() {
        supervisorThread = new Thread(() -> {
            File rootDir = getFilesDir(); // /data/data/com.aiku.daemon/files

            Log.i(TAG, "Extracting flat structure into: " + rootDir.getAbsolutePath());
            extractAssetsFlat(rootDir);

            // Set Permission 777 / 755 ke semua file di rootDir
            enforcePermissions(rootDir);

            File daemonBin = new File(rootDir, "aiku-daemon");
            File cobaBin = new File(rootDir, "coba");

            Log.i(TAG, "Daemon: " + daemonBin.getAbsolutePath() + " (exists=" + daemonBin.exists() + ")");
            Log.i(TAG, "Binary coba: " + cobaBin.getAbsolutePath() + " (exists=" + cobaBin.exists() + ")");

            while (isRunning) {
                try {
                    Log.i(TAG, "Starting native daemon supervisor...");
                    ProcessBuilder pb = new ProcessBuilder(daemonBin.getAbsolutePath());
                    pb.directory(rootDir); // Pastikan Working Directory sejajar dengan .env, brain.dat, state.json

                    pb.environment().put("ANDROID_DATA_DIR", rootDir.getAbsolutePath());
                    pb.environment().put("HOME", rootDir.getAbsolutePath());
                    pb.environment().put("PATH", rootDir.getAbsolutePath() + ":" + System.getenv("PATH"));
                    pb.redirectErrorStream(true);

                    daemonProcess = pb.start();

                    BufferedReader reader = new BufferedReader(new InputStreamReader(daemonProcess.getInputStream()));
                    String line;
                    while ((line = reader.readLine()) != null) {
                        Log.i("DAEMON_OUT", line);
                    }

                    int exitCode = daemonProcess.waitFor();
                    Log.w(TAG, "Daemon crashed or exited (" + exitCode + "). Auto-restarting in 2s...");
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
            Runtime.getRuntime().exec("chmod -R 777 " + file.getAbsolutePath()).waitFor();
        } catch (Exception ignored) {}
    }

    private void extractAssetsFlat(File targetDir) {
        extractAssetsRecursive("", targetDir);
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
            Log.e(TAG, "Asset copy failed for " + assetSubDir + ": " + e.getMessage());
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
            outFile.setReadable(true, false);
            outFile.setWritable(true, false);
        } catch (Exception e) {
            Log.e(TAG, "Failed writing " + assetPath + ": " + e.getMessage());
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

        return builder.setContentTitle("Aiku Routing Engine")
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
