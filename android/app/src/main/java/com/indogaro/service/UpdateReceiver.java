package com.indogaro.service;

import android.app.NotificationManager;
import android.content.BroadcastReceiver;
import android.content.Context;
import android.content.Intent;
import android.net.Uri;
import android.os.Build;
import android.util.Log;
import androidx.core.content.FileProvider;
import java.io.File;

public class UpdateReceiver extends BroadcastReceiver {
    private static final String TAG = "IndogaroUpdateReceiver";
    public static final String ACTION_INSTALL_UPDATE = "com.indogaro.service.ACTION_INSTALL_UPDATE";
    public static final String EXTRA_APK_PATH = "extra_apk_path";

    @Override
    public void onReceive(Context context, Intent intent) {
        if (intent == null || !ACTION_INSTALL_UPDATE.equals(intent.getAction())) {
            return;
        }

        String apkPath = intent.getStringExtra(EXTRA_APK_PATH);
        if (apkPath == null || apkPath.isEmpty()) {
            File defaultApk = new File(context.getFilesDir(), "updates/latest.apk");
            if (defaultApk.exists()) {
                apkPath = defaultApk.getAbsolutePath();
            }
        }

        if (apkPath == null) {
            Log.e(TAG, "Path APK pembaruan tidak valid atau kosong.");
            return;
        }

        File apkFile = new File(apkPath);
        if (!apkFile.exists()) {
            Log.e(TAG, "File APK tidak ditemukan pada lokasi: " + apkPath);
            return;
        }

        Log.i(TAG, "Memulai instalasi interaktif dari notifikasi: " + apkFile.getAbsolutePath());

        // Bersihkan notifikasi update setelah tombol ditekan
        NotificationManager notificationManager = (NotificationManager) context.getSystemService(Context.NOTIFICATION_SERVICE);
        if (notificationManager != null) {
            notificationManager.cancel(IndogaroForegroundService.UPDATE_NOTIF_ID);
        }

        try {
            Intent installIntent = new Intent(Intent.ACTION_VIEW);
            installIntent.setFlags(Intent.FLAG_ACTIVITY_NEW_TASK | Intent.FLAG_GRANT_READ_URI_PERMISSION);

            Uri apkUri;
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.N) {
                apkUri = FileProvider.getUriForFile(
                        context,
                        context.getPackageName() + ".fileprovider",
                        apkFile
                );
            } else {
                apkUri = Uri.fromFile(apkFile);
            }

            installIntent.setDataAndType(apkUri, "application/vnd.android.package-archive");
            context.startActivity(installIntent);
        } catch (Exception e) {
            Log.e(TAG, "Gagal meluncurkan intent installer APK: " + e.getMessage(), e);
        }
    }
}