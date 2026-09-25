#!/usr/bin/env bash
# Builds the SynoCommunity package from the recipe in contrib/spksrc, with
# their framework and in their container — the way their CI builds it.
#
#   tools/build-spksrc.sh                  # x64, DSM 7.2
#   tools/build-spksrc.sh x64 aarch64      # several architectures
#   tools/build-spksrc.sh digests          # checksums of the source tarball, back into contrib/
#
# The result lands in dist/, next to what tools/build-spk.sh makes.
#
# Their framework is a clone of SynoCommunity/spksrc, over 3 GB once the
# toolchains and source tarballs are in. It is a cache, not a part of this
# project: it lives in ~/Library/Caches/dsm-mini/spksrc (or $SPKSRC_DIR), is
# cloned on the first run and reused afterwards — the downloads are what makes
# a second build take minutes rather than an hour. It stays out of the
# repository folder on purpose: gofmt, the IDE and backups would all walk
# through somebody else's three gigabytes, Go toolchain sources included.
# Their master moves on; `git -C <that folder> pull` before a build meant for
# a pull request to them.
#
# The recipe is copied in afresh on every run. A copy kept there goes stale:
# the one in the first clone still had the install wizard after the package
# had dropped it.
#
# Docker has to be running. On an Apple Silicon Mac the container runs under
# emulation, which their documentation asks for with --platform=linux/amd64.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RECIPE="$ROOT/contrib/spksrc"
OUT_DIR="${OUT_DIR:-$ROOT/dist}"
TCVERSION="${TCVERSION:-7.2}"
IMAGE="ghcr.io/synocommunity/spksrc"

if [ -z "${SPKSRC_DIR:-}" ]; then
    if [ -d "$HOME/Library/Caches" ]; then
        SPKSRC_DIR="$HOME/Library/Caches/dsm-mini/spksrc"
    else
        SPKSRC_DIR="${XDG_CACHE_HOME:-$HOME/.cache}/dsm-mini/spksrc"
    fi
fi

docker info >/dev/null 2>&1 || { echo "Docker is not running: start it and try again" >&2; exit 1; }

if [ ! -d "$SPKSRC_DIR/.git" ]; then
    echo "→ cloning SynoCommunity/spksrc into $SPKSRC_DIR"
    mkdir -p "$(dirname "$SPKSRC_DIR")"
    git clone --depth 1 https://github.com/SynoCommunity/spksrc "$SPKSRC_DIR"
fi

echo "→ the recipe from contrib/spksrc"
for part in cross spk; do
    rm -rf "${SPKSRC_DIR:?}/$part/dsm-mini"
    cp -R "$RECIPE/$part/dsm-mini" "$SPKSRC_DIR/$part/dsm-mini"
done

in_container() {
    docker run --rm --platform=linux/amd64 -v "$SPKSRC_DIR":/spksrc -w /spksrc \
        -e TAR_CMD="fakeroot tar" "$IMAGE" bash -c "$1"
}

# `make setup` writes local.mk, once per clone.
SETUP='[ -f local.mk ] || make setup >/dev/null'

if [ "${1:-}" = "digests" ]; then
    in_container "$SETUP && make -C cross/dsm-mini digests"
    cp "$SPKSRC_DIR/cross/dsm-mini/digests" "$RECIPE/cross/dsm-mini/digests"
    echo "done: contrib/spksrc/cross/dsm-mini/digests"
    exit 0
fi

# Their packages/ keeps every build ever made; only what this run built, by
# the version and revision in the recipe, is copied out.
VERS=$(sed -n 's/^SPK_VERS = //p' "$RECIPE/spk/dsm-mini/Makefile")
REV=$(sed -n 's/^SPK_REV = //p' "$RECIPE/spk/dsm-mini/Makefile")

mkdir -p "$OUT_DIR"
for arch in "${@:-x64}"; do
    echo "→ building for $arch-$TCVERSION"
    in_container "$SETUP && make -C spk/dsm-mini ARCH=$arch TCVERSION=$TCVERSION"
    spk="$SPKSRC_DIR/packages/dsm-mini_$arch-${TCVERSION}_$VERS-$REV.spk"
    cp "$spk" "$OUT_DIR/"
    echo "done: $OUT_DIR/$(basename "$spk")"
done
