package com.aiku.daemon;

import android.app.Activity;
import android.content.Intent;
import android.os.Build;
import android.os.Bundle;
import android.widget.Toast;

public class MainActivity extends Activity {
    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);

        // Jalankan Foreground Service
        Intent serviceIntent = new Intent(this, AikuForegroundService.class);
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            startForegroundService(serviceIntent);
        } else {
            startService(serviceIntent);
        }

        Toast.makeText(this, "Aiku Service Started in Background", Toast.LENGTH_SHORT).show();

        // Langsung sembunyikan aplikasi ke background (tanpa mematikan service)
        moveTaskToBack(true);
        finish();
    }
}
