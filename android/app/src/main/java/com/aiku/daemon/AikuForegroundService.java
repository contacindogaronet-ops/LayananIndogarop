package com.aiku.daemon;

import android.app.Notification;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.app.PendingIntent;
import android.app.Service;
import android.content.Intent;
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
        Notification notification = buildNotification("Aiku Daemon Active (Running in background)");
        startForeground(1001, notification);

        if (!isRunning) {
            isRunning = true;
            startDaemonSupervisor();
        }

        return START_STICKY; // Auto restart jika dimatikan OS
    }

    private void startDaemonSupervisor() {
        supervisorThread = new Thread(() -> {
            File filesDir = getFilesDir();
            File binFile = new File(filesDir, "aiku-daemon");

            // Extract binary & configs dari APK assets
            extractAssets(filesDir);
            binFile.setExecutable(true, false);

            while (isRunning) {
                try {
                    Log.i(TAG, "Starting native Aiku daemon process...");
                    ProcessBuilder pb = new ProcessBuilder(binFile.getAbsolutePath());
                    pb.directory(filesDir);
                    pb.environment().put("ANDROID_DATA_DIR", filesDir.getAbsolutePath());
                    pb.redirectErrorStream(true);

                    daemonProcess = pb.start();
                    int exitCode = daemonProcess.waitFor();
                    Log.w(TAG, "Daemon stopped with code " + exitCode + ". Restarting in 2 seconds...");
                } catch (Exception e) {
                    Log.e(TAG, "Daemon error: " + e.getMessage(), e);
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

    private void extractAssets(File destDir) {
        try {
            String[] files = getAssets().list("");
            if (files != null) {
                for (String filename : files) {
                    if (filename.equals("images") || filename.equals("webkit")) continue;
                    File outFile = new File(destDir, filename);
                    if (!outFile.exists() || filename.equals("aiku-daemon")) {
                        try (InputStream in = getAssets().open(filename);
                             OutputStream out = new FileOutputStream(outFile)) {
                            byte[] buffer = new byte[8192];
                            int read;
                            while ((read = in.read(buffer)) != -1) {
                                out.write(buffer, 0, read);
                            }
                        }
                    }
                }
            }
        } catch (Exception e) {
            Log.e(TAG, "Asset extraction error: " + e.getMessage());
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
