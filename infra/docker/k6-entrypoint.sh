#!/bin/sh
set -euo pipefail

ORIGINAL_PATH="${HERO_IMAGE_PATH:-}"
MAX_BYTES="${HERO_IMAGE_MAX_BYTES:-5242880}"
MAX_DIM="${HERO_IMAGE_MAX_DIMENSION:-1920}"
QUALITY="${HERO_IMAGE_QUALITY:-85}"
TMP_PATH="/tmp/hero-image-resized.jpg"

resize_required=false

if [ -n "$ORIGINAL_PATH" ] && [ -f "$ORIGINAL_PATH" ]; then
    current_size=0
    if current_size=$(stat -c %s "$ORIGINAL_PATH" 2>/dev/null); then
        :
    elif current_size=$(wc -c < "$ORIGINAL_PATH" 2>/dev/null); then
        :
    else
        current_size=0
    fi
    if [ "$current_size" -gt "$MAX_BYTES" ]; then
        resize_required=true
    fi
fi

if $resize_required; then
    echo "[k6-entrypoint] Resizing hero image to stay under ${MAX_BYTES} bytes" >&2
    if ! command -v convert >/dev/null 2>&1; then
        echo "ImageMagick convert utility missing" >&2
        exit 1
    fi
    convert "$ORIGINAL_PATH" -strip -resize "${MAX_DIM}x${MAX_DIM}>" -quality "$QUALITY" -define "jpeg:extent=${MAX_BYTES}" "$TMP_PATH"
    if [ ! -s "$TMP_PATH" ]; then
        echo "Failed to resize image" >&2
        exit 1
    fi
    if new_size=$(stat -c %s "$TMP_PATH" 2>/dev/null); then
        :
    elif new_size=$(wc -c < "$TMP_PATH" 2>/dev/null); then
        :
    else
        new_size=0
    fi
    if [ "$new_size" -gt "$MAX_BYTES" ]; then
        echo "[k6-entrypoint] Resized image still exceeds limit (${new_size} bytes)" >&2
        exit 1
    fi
    HERO_IMAGE_PATH="$TMP_PATH"
    export HERO_IMAGE_PATH
fi

exec k6 "$@"
