package com.indogaro.companion;

import android.content.ComponentName;
import android.content.Intent;
import android.graphics.drawable.Icon;
import android.os.Build;
import android.service.quicksettings.Tile;
import android.service.quicksettings.TileService;
import androidx.annotation.RequiresApi;

@RequiresApi(api = Build.VERSION_CODES.N)
public class IndogaroTileService extends TileService {

    private static final String CORE_PACKAGE = "com.indogaro.service";
    private static final String CORE_SERVICE = "com.indogaro.service.IndogaroForegroundService";
    private boolean isRunning = false;

    @Override
    public void onStartListening() {
        super.onStartListening();
        updateTileState();
    }

    @Override
    public void onClick() {
        super.onClick();
        if (isRunning) {
            stopCoreService();
            isRunning = false;
        } else {
            startCoreService();
            isRunning = true;
        }
        updateTileState();
    }

    private void startCoreService() {
        Intent intent = new Intent();
        intent.setComponent(new ComponentName(CORE_PACKAGE, CORE_SERVICE));
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            startForegroundService(intent);
        } else {
            startService(intent);
        }
    }

    private void stopCoreService() {
        Intent intent = new Intent();
        intent.setComponent(new ComponentName(CORE_PACKAGE, CORE_SERVICE));
        stopService(intent);
    }

    private void updateTileState() {
        Tile tile = getQsTile();
        if (tile != null) {
            tile.setState(isRunning ? Tile.STATE_ACTIVE : Tile.STATE_INACTIVE);
            tile.setLabel(isRunning ? "Indogaro (ON)" : "Indogaro (OFF)");
            tile.updateTile();
        }
    }
}
