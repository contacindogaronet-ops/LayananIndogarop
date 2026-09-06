#!/usr/bin/env bash
set -e

REPO_DIR="/data/data/com.termux/files/home/aiku-daemon"
cd "$REPO_DIR"

echo "==> [AICLO] Menyiapkan safe directory Git..."
git config --global --add safe.directory "$REPO_DIR" 2>/dev/null || true

echo "==> [AICLO] Staging seluruh berkas..."
git add -A

# Periksa status working tree
if git diff-index --quiet HEAD -- 2>/dev/null; then
    echo "==> [AICLO] Tidak ada berkas baru yang belum di-commit."
else
    COMMIT_MSG="feat(core): autonomous sync and update QS Tile, OTA notification, and core daemon"
    echo "==> [AICLO] Membuat commit: ${COMMIT_MSG}"
    git commit -m "${COMMIT_MSG}"
fi

# Deteksi branch saat ini
BRANCH=$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "main")
if [ "$BRANCH" = "HEAD" ] || [ -z "$BRANCH" ]; then
    BRANCH="main"
fi

# Deteksi remote target
REMOTE=$(git remote | head -n 1)
if [ -z "$REMOTE" ]; then
    REMOTE="origin"
fi

echo "==> [AICLO] Mengeksekusi push ke ${REMOTE}/${BRANCH}..."
git push -u "$REMOTE" "$BRANCH"

echo "==> [AICLO SUCCESS] Seluruh repositori berhasil di-push!"