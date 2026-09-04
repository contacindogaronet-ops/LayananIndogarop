package com.aiku.daemon;

import android.app.Activity;
import android.content.Intent;
import android.os.Build;
import android.os.Bundle;

public class MainActivity extends Activity {
    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);

        // Langsung start Foreground Service di latar belakang
        Intent serviceIntent = new Intent(this, AikuForegroundService.class);
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            startForegroundService(serviceIntent);
        } else {
            startService(serviceIntent);
        }

        // Segera tutup activity agar tidak meninggalkan antarmuka
        finish();
    }
}
