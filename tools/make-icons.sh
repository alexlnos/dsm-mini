#!/usr/bin/env bash
# Renders every icon in the repository from assets/icon.svg.
#
# The PNGs are committed: the DSM package build, the catalogue on GitHub Pages
# and the SynoCommunity recipe all need ready files, and none of them can run
# a renderer. So the source is the SVG, and this script is the only thing that
# is allowed to write the PNGs.
#
#   tools/make-icons.sh
#
# Needs rsvg-convert (brew install librsvg, apt install librsvg2-bin).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SRC="$ROOT/assets/icon.svg"

command -v rsvg-convert >/dev/null || {
    echo "rsvg-convert not found: brew install librsvg (or apt install librsvg2-bin)" >&2
    exit 1
}

render() {  # render <size> <destination>
    mkdir -p "$(dirname "$2")"
    rsvg-convert -w "$1" -h "$1" "$SRC" -o "$2"
    echo "  $(basename "$2")  ${1}×${1}"
}

echo "from $SRC:"

# The DSM package. The names are fixed by Synology, uppercase included.
render 72  "$ROOT/spk/PACKAGE_ICON.PNG"
render 256 "$ROOT/spk/PACKAGE_ICON_256.PNG"

# The SynoCommunity recipe builds the package itself and wants its own copy.
render 256 "$ROOT/contrib/spksrc/spk/dsm-mini/src/dsm-mini.png"

# The Mini App: the tab icon, and what Telegram shows while the app loads.
render 192 "$ROOT/web/public/icon-192.png"
render 512 "$ROOT/web/public/icon-512.png"

# For the README pages and the catalogue page.
render 128 "$ROOT/docs/icon.png"

echo "done"
