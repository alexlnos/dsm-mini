#!/usr/bin/env bash
# Builds an .spk package for the Synology Package Center.
#
# Synology's official toolkit (pkgscripts-ng) needs a chroot with their build
# environment. We do not need it: the binary is static and depends on nothing,
# so the package is built with plain tar — exactly the way spksrc does it.
#
#   tools/build-spk.sh amd64 1.2.3
#
# Result: dist/dsm-mini-1.2.3-amd64.spk
set -euo pipefail

ARCH="${1:-amd64}"
VERSION="${2:-0.0.0}"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT_DIR="${OUT_DIR:-$ROOT/dist}"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

# The arch values are Synology platform names, not processor names. One
# package covers every model of one architecture: the binary is the same.
case "$ARCH" in
    amd64)
        GOARCH=amd64
        SYNO_ARCH="x86_64 apollolake avoton braswell broadwell broadwellnk broadwellnkv2 broadwellntbap bromolow cedarview denverton epyc7002 epyc7003 epyc7003ntb geminilake geminilakenk grantley kvmx64 purley r1000 r1000nk v1000 v1000nk"
        ;;
    arm64)
        GOARCH=arm64
        SYNO_ARCH="aarch64 rtd1296 rtd1619b armada37xx"
        ;;
    *)
        echo "unknown architecture: $ARCH (amd64 or arm64 expected)" >&2
        exit 1
        ;;
esac

# DSM understands only digits and the . - _ separators in a version
SPK_VERSION="$(printf '%s' "$VERSION" | sed 's/^v//')"
case "$SPK_VERSION" in
    *-*) : ;;                      # a build number is already there
    *) SPK_VERSION="$SPK_VERSION-1" ;;
esac

echo "→ building the binary ($GOARCH)"
mkdir -p "$WORK/staging/bin"
CGO_ENABLED=0 GOOS=linux GOARCH="$GOARCH" go build -C "$ROOT" -trimpath \
    -ldflags "-s -w -X main.version=$SPK_VERSION" \
    -o "$WORK/staging/bin/dsm-mini" ./cmd/dsm-mini

echo "→ package.tgz"
( cd "$WORK/staging" && tar cpzf "$WORK/package.tgz" --owner=root --group=root . )

echo "→ INFO"
cat > "$WORK/INFO" <<EOF
package="dsm-mini"
version="$SPK_VERSION"
os_min_ver="7.0-40000"
displayname="dsm-mini"
maintainer="alexlnos"
maintainer_url="https://github.com/alexlnos/dsm-mini"
support_url="https://github.com/alexlnos/dsm-mini/issues"
arch="$SYNO_ARCH"
thirdparty="yes"
startable="yes"
silent_install="no"
silent_upgrade="no"
silent_uninstall="no"
ctl_stop="yes"
EOF

# The listing text comes from assets/store.json so that the package and the
# catalogue on GitHub Pages cannot drift apart. Package Center picks the
# description matching the DSM language and falls back to `description`.
python3 - "$ROOT/assets/store.json" >> "$WORK/INFO" <<'PYEOF'
import json, sys

store = json.load(open(sys.argv[1], encoding="utf-8"))

def line(key, value):
    # INFO is key="value"; a stray quote would cut the value short.
    return '%s="%s"' % (key, value.replace('"', "'"))

print(line("description", store["description"]["enu"]))
for lang, text in store["description"].items():
    print(line("description_%s" % lang, text))
print(line("changelog", store["changelog"]))
PYEOF

cp -r "$ROOT/spk/scripts" "$ROOT/spk/conf" "$ROOT/spk/WIZARD_UIFILES" "$WORK/"
cp "$ROOT/spk/PACKAGE_ICON.PNG" "$ROOT/spk/PACKAGE_ICON_256.PNG" "$WORK/"
cp "$ROOT/LICENSE" "$WORK/LICENSE"
chmod 755 "$WORK"/scripts/*

echo "→ .spk"
mkdir -p "$OUT_DIR"
SPK="$OUT_DIR/dsm-mini-$SPK_VERSION-$ARCH.spk"
# The file order matches spksrc: DSM reads INFO without unpacking everything.
( cd "$WORK" && tar cpf "$SPK" --owner=root --group=root \
    package.tgz INFO scripts PACKAGE_ICON.PNG PACKAGE_ICON_256.PNG WIZARD_UIFILES conf LICENSE )

echo "done: $SPK ($(du -h "$SPK" | cut -f1))"
