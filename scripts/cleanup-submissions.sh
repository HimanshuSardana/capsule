#!/bin/bash

cleanup_dir() {
    local dir="$1"
    if [[ ! -d "$dir" ]]; then
        echo "Skipping $dir (not found)"
        return
    fi

    echo "Cleaning $dir"

    local mounts=$(find "$dir" -mount 2>/dev/null)
    for m in $mounts; do
        if mountpoint -q "$m" 2>/dev/null; then
            echo "  Unmounting $m"
            sudo umount -f "$m" 2>/dev/null || sudo umount "$m" 2>/dev/null
        fi
    done

    sudo rm -rf "$dir" 2>/dev/null && echo "  Removed $dir" || {
        echo "  Failed to remove $dir"
    }
}

BASE="/tmp/capsule/submissions"
cleanup_dir "$BASE"

PROJECT_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cleanup_dir "$PROJECT_ROOT/python-container"
cleanup_dir "$PROJECT_ROOT/testdir"
cleanup_dir "$PROJECT_ROOT/testdir2"

echo "Done."
