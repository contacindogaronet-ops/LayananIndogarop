#!/usr/bin/env bash
set -e

echo "==> [AICLO] Menyiapkan sinkronisasi Git Repository..."

# Pastikan berada di direktori root project
cd "$(dirname "$0")"

# Stage seluruh file yang diubah dan file baru
git add -A

# Periksa status working tree
if git diff-index --quiet HEAD --; then
    echo "==> [AICLO] Tidak ada perubahan baru untuk di-commit."
else
    COMMIT_MSG="feat(core): update license, setup docs, interactive notification OTA and QS tile extension"
    echo "==> [AICLO] Melakukan commit: ${COMMIT_MSG}"
    git commit -m "${COMMIT_MSG}"
fi

# Dapatkan nama branch aktif saat ini
CURRENT_BRANCH=$(git rev-parse --abbrev-ref HEAD)
if [ -z "$CURRENT_BRANCH" ]; then
    CURRENT_BRANCH="main"
fi

echo "==> [AICLO] Melakukan push ke origin/${CURRENT_BRANCH}..."
git push origin "$CURRENT_BRANCH"

echo "==> [AICLO] Sinkronisasi Git selesai dengan sukses!"