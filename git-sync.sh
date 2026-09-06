#!/usr/bin/env bash
set -e

echo "==> [AICLO] Memulai pipeline Git Autonomous Engine..."

# Pindah ke direktori root repository
cd "$(dirname "$0")"

# 1. Pastikan safe directory untuk Termux / Android environment
git config --global --add safe.directory "$PWD" 2>/dev/null || true

# 2. Stage seluruh file (termasuk file baru, modifikasi, dan untracked)
echo "==> [AICLO] Menjalankan: git add -A"
git add -A

# 3. Cek perubahan di working tree & staging area
if git diff-index --quiet HEAD -- 2>/dev/null; then
    echo "==> [AICLO] Tidak ada perubahan baru pada working tree untuk di-commit."
else
    COMMIT_MSG="feat(core): update license, setup docs, interactive OTA notification, and QS tile service"
    echo "==> [AICLO] Melakukan commit: ${COMMIT_MSG}"
    git commit -m "${COMMIT_MSG}"
fi

# 4. Deteksi branch aktif saat ini
CURRENT_BRANCH=$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "main")
if [ "$CURRENT_BRANCH" = "HEAD" ] || [ -z "$CURRENT_BRANCH" ]; then
    CURRENT_BRANCH="main"
fi

# 5. Deteksi remote yang terkonfigurasi (default: origin)
REMOTE_NAME=$(git remote | head -n 1)
if [ -z "$REMOTE_NAME" ]; then
    echo "==> [AICLO ERROR] Tidak ditemukan git remote repository!"
    exit 1
fi

echo "==> [AICLO] Branch aktif: ${CURRENT_BRANCH} | Remote: ${REMOTE_NAME}"

# 6. Fetch & Push ke Remote Repository
echo "==> [AICLO] Mendorong commit ke ${REMOTE_NAME}/${CURRENT_BRANCH}..."
git push -u "$REMOTE_NAME" "$CURRENT_BRANCH"

echo "==> [AICLO SUCCESS] Semua perubahan telah berhasil di-push ke remote repository!"