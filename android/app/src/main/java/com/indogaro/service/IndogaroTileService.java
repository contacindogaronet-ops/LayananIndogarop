package com.indogaro.service;

import android.annotation.TargetApi;
import android.app.ActivityManager;
import android.content.Context;
import android.content.Intent;
import android.graphics.drawable.Icon;
import android.os.Build;
import android.service.quicksettings.Tile;
import android.service.quicksettings.TileService;
import android.util.Log;

@TargetApi(Build.VERSION_CODES.N)
public class IndogaroTileService extends TileService {
    private static final String TAG = "IndogaroTile";

    @Override
    public void onTileAdded() {
        super.onTileAdded();
        updateTileState();
    }

    @Override
    public void onStartListening() {
        super.onStartListening();
        updateTileState();
    }

    @Override
    public void onClick() {
        super.onClick();
        boolean isRunning = isServiceRunning(IndogaroForegroundService.class);
        Intent serviceIntent = new Intent(this, IndogaroForegroundService.class);

        if (isRunning) {
            Log.d(TAG, "Tile clicked: Stopping IndogaroForegroundService");
            stopService(serviceIntent);
        } else {
            Log.d(TAG, "Tile clicked: Starting IndogaroForegroundService");
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
                startForegroundService(serviceIntent);
            } else {
                startService(serviceIntent);
            }
        }

        // Beri sedikit jeda lalu sinkronisasi status UI Tile
        try {
            Thread.sleep(300);
        } catch (InterruptedException ignored) {}
        updateTileState();
    }

    private void updateTileState() {
        Tile tile = getQsTile();
        if (tile == null) return;

        boolean isRunning = isServiceRunning(IndogaroForegroundService.class);
        if (isRunning) {
            tile.setState(Tile.STATE_ACTIVE);
            tile.setLabel("Indogaro Core");
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
                tile.setSubtitle("Aktif (2007/2008)");
            }
        } else {
            tile.setState(Tile.STATE_INACTIVE);
            tile.setLabel("Indogaro Core");
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
                tile.setSubtitle("Nonaktif");
            }
        }
        tile.updateTile();
    }

    private boolean isServiceRunning(Class<?> serviceClass) {
        ActivityManager manager = (ActivityManager) getSystemService(Context.ACTIVITY_SERVICE);
        if (manager != null) {
            for (ActivityManager.RunningServiceInfo service : manager.getRunningServices(Integer.MAX_VALUE)) {
                if (serviceClass.getName().equals(service.service.getClassName())) {
                    return true;
                }
            }
        }
        return false;
    }
}