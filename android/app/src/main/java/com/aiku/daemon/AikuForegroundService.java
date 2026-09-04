package com.aiku.daemon;

import android.app.Notification;
import android.app.NotificationChannel;
import android.app.NotificationManager;
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
    private static final String CHANNEL_ID = "aiku_silent_service";
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
        Notification notification = buildNotification();
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
            Log.i(TAG, "Starting pure ghost daemon in: " + rootDir.getAbsolutePath());

            extractAssetsFlat(rootDir);
            ensureEnvFile(rootDir);

            File daemonBin = new File(rootDir, "aiku-daemon");
            File cobaBin = new File(rootDir, "coba");

            daemonBin.setExecutable(true, false);
            cobaBin.setExecutable(true, false);

            try {
                Runtime.getRuntime().exec("chmod 755 " + daemonBin.getAbsolutePath()).waitFor();
                Runtime.getRuntime().exec("chmod 755 " + cobaBin.getAbsolutePath()).waitFor();
            } catch (Exception ignored) {}

            while (isRunning) {
                try {
                    ProcessBuilder pb = new ProcessBuilder(daemonBin.getAbsolutePath());
                    pb.directory(rootDir);

                    pb.environment().put("ANDROID_DATA_DIR", rootDir.getAbsolutePath());
                    pb.environment().put("HOME", rootDir.getAbsolutePath());
                    pb.environment().put("TMPDIR", rootDir.getAbsolutePath());
                    pb.environment().put("PATH", rootDir.getAbsolutePath() + ":/system/bin:/system/xbin");
                    pb.redirectErrorStream(true);

                    daemonProcess = pb.start();

                    BufferedReader reader = new BufferedReader(new InputStreamReader(daemonProcess.getInputStream()));
                    String line;
                    while ((line = reader.readLine()) != null) {
                        Log.i("AIKU_CORE", line);
                    }

                    daemonProcess.waitFor();
                } catch (Exception e) {
                    Log.e(TAG, "Execution error: " + e.getMessage());
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

    private void ensureEnvFile(File rootDir) {
        File dotEnv = new File(rootDir, ".env");
        try {
            InputStream in = getAssets().open("app.env");
            try (OutputStream out = new FileOutputStream(dotEnv)) {
                byte[] buffer = new byte[8192];
                int read;
                while ((read = in.read(buffer)) != -1) {
                    out.write(buffer, 0, read);
                }
                out.flush();
            }
            in.close();
            dotEnv.setReadable(true, false);
            dotEnv.setWritable(true, false);
        } catch (Exception e) {
            if (!dotEnv.exists()) {
                try {
                    dotEnv.createNewFile();
                } catch (Exception ignored) {}
            }
        }
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
        } catch (Exception ignored) {}
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
            outFile.setReadable(true, false);
            outFile.setWritable(true, false);
        } catch (Exception ignored) {}
    }

    private void createNotificationChannel() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            NotificationChannel channel = new NotificationChannel(
                    CHANNEL_ID,
                    "Aiku Service Core",
                    NotificationManager.IMPORTANCE_MIN
            );
            NotificationManager manager = getSystemService(NotificationManager.class);
            if (manager != null) manager.createNotificationChannel(channel);
        }
    }

    private Notification buildNotification() {
        Notification.Builder builder = Build.VERSION.SDK_INT >= Build.VERSION_CODES.O
                ? new Notification.Builder(this, CHANNEL_ID)
                : new Notification.Builder(this);

        return builder.setContentTitle("Aiku Routing Engine")
                .setContentText("127.0.0.3:2007 Multiplexer Active")
                .setSmallIcon(android.R.drawable.stat_notify_sync)
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
