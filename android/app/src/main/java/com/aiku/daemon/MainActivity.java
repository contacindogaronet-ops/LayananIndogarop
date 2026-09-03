package com.aiku.daemon;

import android.app.Activity;
import android.content.Intent;
import android.os.Build;
import android.os.Bundle;
import android.os.Handler;
import android.os.Looper;
import android.widget.Button;
import android.widget.ScrollView;
import android.widget.TextView;

import java.io.BufferedReader;
import java.io.File;
import java.io.FileReader;
import java.net.InetSocketAddress;
import java.net.Socket;

public class MainActivity extends Activity {
    private TextView tvStatus;
    private TextView tvLogs;
    private ScrollView scrollLogs;
    private final Handler handler = new Handler(Looper.getMainLooper());
    private File logFile;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.activity_main);

        tvStatus = findViewById(R.id.tvStatus);
        tvLogs = findViewById(R.id.tvLogs);
        scrollLogs = findViewById(R.id.scrollLogs);
        Button btnRefresh = findViewById(R.id.btnRefreshLogs);
        Button btnTestPort = findViewById(R.id.btnTestPort);
        Button btnRestart = findViewById(R.id.btnRestart);

        logFile = new File(getFilesDir(), "app.log");

        startDaemonService();

        btnRefresh.setOnClickListener(v -> readLogs());
        btnTestPort.setOnClickListener(v -> testPorts());
        btnRestart.setOnClickListener(v -> {
            stopService(new Intent(this, AikuForegroundService.class));
            handler.postDelayed(this::startDaemonService, 1000);
        });

        // Auto Refresh Logs setiap 2 detik
        handler.post(new Runnable() {
            @Override
            public void run() {
                readLogs();
                handler.postDelayed(this, 2000);
            }
        });
    }

    private void startDaemonService() {
        Intent serviceIntent = new Intent(this, AikuForegroundService.class);
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            startForegroundService(serviceIntent);
        } else {
            startService(serviceIntent);
        }
        tvStatus.setText("Status: Service Started | FilesDir: " + getFilesDir().getAbsolutePath());
    }

    private void readLogs() {
        if (!logFile.exists()) {
            tvLogs.setText("Menunggu proses menulis log...");
            return;
        }

        StringBuilder sb = new StringBuilder();
        try (BufferedReader br = new BufferedReader(new FileReader(logFile))) {
            String line;
            while ((line = br.readLine()) != null) {
                sb.append(line).append("\n");
            }
            tvLogs.setText(sb.toString());
            scrollLogs.post(() -> scrollLogs.fullScroll(ScrollView.FOCUS_DOWN));
        } catch (Exception e) {
            tvLogs.setText("Error membaca log: " + e.getMessage());
        }
    }

    private void testPorts() {
        new Thread(() -> {
            boolean port8080 = checkSocket("127.0.0.1", 8080);
            boolean port2007 = checkSocket("127.0.0.3", 2007);

            String res = "Test Result:\n- 127.0.0.1:8080 (Daemon API): " + (port8080 ? "ONLINE [OK]" : "REFUSED / DOWN")
                    + "\n- 127.0.0.3:2007 (Binary Coba): " + (port2007 ? "ONLINE [OK]" : "REFUSED / DOWN");

            handler.post(() -> tvStatus.setText(res));
        }).start();
    }

    private boolean checkSocket(String host, int port) {
        try (Socket s = new Socket()) {
            s.connect(new InetSocketAddress(host, port), 1000);
            return true;
        } catch (Exception e) {
            return false;
        }
    }
}
