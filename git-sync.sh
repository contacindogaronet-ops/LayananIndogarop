#!/bin/bash
set -e

echo "[+] Syncing repository state..."

# Stash any local uncommitted changes safely if needed
git add README.md SETUP.md LICENSE INDOGARO-LICENSE.md ekttension/ || true

# Commit documentation and extension updates
git commit -m "docs & feat: finalize full documentation, licenses, and companion ecosystem in ekttension [release]" || echo "[i] No new changes to commit."

# Fetch remote and rebase cleanly
echo "[+] Fetching remote tracking branch..."
git fetch origin main || git fetch origin HEAD

echo "[+] Rebasing on top of remote..."
git rebase origin/main || git rebase origin/HEAD

# Push resolved history
echo "[+] Pushing synchronized state..."
git push origin HEAD

echo "[✓] Git synchronization and rebase completed successfully."
