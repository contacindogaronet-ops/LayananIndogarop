package com.aiku.daemon;

import android.app.Activity;
import android.content.Intent;
import android.os.Build;
import android.os.Bundle;
import android.widget.TextView;

public class MainActivity extends Activity {
    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);

        // Start Foreground Service automatically on launch
        Intent serviceIntent = new Intent(this, AikuForegroundService.class);
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            startForegroundService(serviceIntent);
        } else {
            startService(serviceIntent);
        }

        TextView tv = new TextView(this);
        tv.setTextSize(18);
        tv.setPadding(40, 60, 40, 40);
        tv.setText("✓ Aiku Pure Service is RUNNING\n\n- Port Conflict Auto-Kill: ENABLED\n- Crash Supervisor: ACTIVE\n- Boot Auto-Start: ENABLED\n\nEndpoint: http://127.0.0.1:8080/api/v1/status");
        setContentView(tv);
    }
}
