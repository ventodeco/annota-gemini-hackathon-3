#!/usr/bin/env bash
# Renders the evaluation HTML samples to fixed PNG fixtures with headless Chrome.
# The committed PNGs are canonical; re-rendering on another OS or font set changes pixels.
set -euo pipefail
dir="$(cd "$(dirname "$0")" && pwd)"
chrome="${CHROME:-/Applications/Google Chrome.app/Contents/MacOS/Google Chrome}"
mkdir -p "$dir/images"
for html in "$dir"/samples/*.html; do
  name="$(basename "$html" .html)"
  "$chrome" --headless=new --disable-gpu --hide-scrollbars --force-device-scale-factor=1 \
    --window-size=900,1200 --screenshot="$dir/images/$name.png" "file://$html" >/dev/null 2>&1
  echo "rendered images/$name.png"
done
