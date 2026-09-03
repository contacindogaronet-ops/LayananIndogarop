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
import java.io.FileWriter;
import java.io.InputStream;
import java.io.InputStreamReader;
import java.io.OutputStream;
import java.text.SimpleDateFormat;
import java.util.Date;
import java.util.Locale;

public class AikuForegroundService extends Service {
    private static final String TAG = "AikuCore";
    private static final String CHANNEL_ID = "aiku_service_channel";
    private Process daemonProcess;
    private Thread supervisorThread;
    private PowerManager.WakeLock wakeLock;
    private volatile boolean isRunning = false;
    private File logFile;

    @Override
    public void onCreate() {
        super.onCreate();
        createNotificationChannel();
        logFile = new File(getFilesDir(), "app.log");

        PowerManager pm = (PowerManager) getSystemService(POWER_SERVICE);
        if (pm != null) {
            wakeLock = pm.newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "AikuDaemon:WakeLock");
            wakeLock.acquire();
        }
    }

    public synchronized void appendLog(String tag, String message) {
        String time = new SimpleDateFormat("HH:mm:ss", Locale.getDefault()).format(new Date());
        String logLine = "[" + time + "] [" + tag + "] " + message + "\n";
        Log.i(tag, message);
        try (FileWriter fw = new FileWriter(logFile, true)) {
            fw.write(logLine);
        } catch (Exception ignored) {}
    }

    @Override
    public int onStartCommand(Intent intent, int flags, int startId) {
        Notification notification = buildNotification("Aiku Service Engine Active (Debugging)");
        startForeground(1001, notification);

        if (!isRunning) {
            isRunning = true;
            startDaemonSupervisor();
        }

        return START_STICKY;
    }

    private void startDaemonSupervisor() {
        supervisorThread = new Thread(() -> {
            File rootDir = getFilesDir();
            appendLog("INIT", "Sandbox target: " + rootDir.getAbsolutePath());

            // 1. Ekstrak Semua File
            extractAssetsFlat(rootDir);

            // 2. Chmod Eksekusi
            enforcePermissions(rootDir);

            File daemonBin = new File(rootDir, "aiku-daemon");
            File cobaBin = new File(rootDir, "coba");

            appendLog("CHECK", "aiku-daemon: " + (daemonBin.exists() ? "OK (" + daemonBin.length() + " bytes)" : "MISSING"));
            appendLog("CHECK", "coba: " + (cobaBin.exists() ? "OK (" + cobaBin.length() + " bytes)" : "MISSING"));
            appendLog("CHECK", ".env: " + (new File(rootDir, ".env").exists() ? "OK" : "MISSING"));
            appendLog("CHECK", "state.json: " + (new File(rootDir, "state.json").exists() ? "OK" : "MISSING"));
            appendLog("CHECK", "brain.dat: " + (new File(rootDir, "brain.dat").exists() ? "OK" : "MISSING"));

            while (isRunning) {
                try {
                    appendLog("EXEC", "Spawning aiku-daemon process...");
                    ProcessBuilder pb = new ProcessBuilder(daemonBin.getAbsolutePath());
                    pb.directory(rootDir);

                    pb.environment().put("ANDROID_DATA_DIR", rootDir.getAbsolutePath());
                    pb.environment().put("HOME", rootDir.getAbsolutePath());
                    pb.environment().put("PATH", rootDir.getAbsolutePath() + ":" + System.getenv("PATH"));
                    pb.redirectErrorStream(true);

                    daemonProcess = pb.start();

                    BufferedReader reader = new BufferedReader(new InputStreamReader(daemonProcess.getInputStream()));
                    String line;
                    while ((line = reader.readLine()) != null) {
                        appendLog("DAEMON", line);
                    }

                    int exitCode = daemonProcess.waitFor();
                    appendLog("WARN", "aiku-daemon stopped with exit code " + exitCode + ". Restarting in 2s...");
                } catch (Exception e) {
                    appendLog("ERROR", "Execution failed: " + e.getMessage());
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
            Runtime.getRuntime().exec("chmod 777 " + file.getAbsolutePath()).waitFor();
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
            appendLog("ASSET_ERR", "Failed on " + assetSubDir + ": " + e.getMessage());
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
            appendLog("COPY_ERR", assetPath + ": " + e.getMessage());
        }
    }

    private void createNotificationChannel() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            NotificationChannel channel = new NotificationChannel(
                    CHANNEL_ID,
                    "Aiku Service Channel",
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

        return builder.setContentTitle("Aiku Service Engine")
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
