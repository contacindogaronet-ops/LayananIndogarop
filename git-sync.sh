#!/usr/bin/env bash
set -e

REPO_DIR="/data/data/com.termux/files/home/aiku-daemon"
cd "$REPO_DIR"

echo "==> [AICLO] Menjalankan go mod tidy..."
go mod tidy

echo "==> [AICLO] Menjalankan test compile lokal..."
go build -v -o /dev/null cmd/daemon/main.go

echo "==> [AICLO] Commit dan push perbaikan..."
git add -A
git commit -m "fix(daemon): implement StartInPlaceHotUpdater and upgrade Go module dependencies" || true

BRANCH=$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "main")
REMOTE=$(git remote | head -n 1 || echo "origin")

git push -u "$REMOTE" "$BRANCH"
echo "==> [AICLO SUCCESS] Fix berhasil diuji dan di-push ke repository!"