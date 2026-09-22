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

# macOS tar writes an AppleDouble companion next to every file whose extended
# attributes it cannot store in the entry itself — `._INFO` beside `INFO`, and
# so on. It then hides them again when it reads the archive back, so `tar tvf`
# showed a clean package while half of its forty entries were these. The first
# entry of the archive was `._package.tgz`, and that is what Package Center's
# parser met where it expected the package: it refused the download as "not a
# package" (error 4521) while the very same file, uploaded by hand, installed
# without a word. GNU tar on Linux ignores the variable, so the build produces
# the same archive on either machine.
export COPYFILE_DISABLE=1

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

# DSM understands only digits and the . - _ separators in a version. A build
# number after a hyphen is allowed but not added here: "1.0.2-1" with a build
# number frozen at 1 told nobody anything, and read worse than "1.0.2".
SPK_VERSION="$(printf '%s' "$VERSION" | sed 's/^v//')"

echo "→ building the binary ($GOARCH)"
mkdir -p "$WORK/staging/bin"
CGO_ENABLED=0 GOOS=linux GOARCH="$GOARCH" go build -C "$ROOT" -trimpath \
    -ldflags "-s -w -X main.version=$SPK_VERSION" \
    -o "$WORK/staging/bin/dsm-mini" ./cmd/dsm-mini

echo "→ package.tgz"
( cd "$WORK/staging" && tar cpzf "$WORK/package.tgz" --format=ustar --owner=root --group=root . )

# The md5 of package.tgz goes into INFO as `checksum`. Package Center verifies
# it when installing from a package source, and a package without the field is
# refused as "not a package" — error 4521, which says nothing about a missing
# checksum. Installing the very same .spk by hand skips the check, so the
# package looked fine for months. Both third-party packages that do install
# from a source — SynoCommunity's and Xpenology's — carry it.
PKG_CHECKSUM="$(md5 -q "$WORK/package.tgz" 2>/dev/null || md5sum "$WORK/package.tgz" | cut -d" " -f1)"

echo "→ INFO"
cat > "$WORK/INFO" <<EOF
# package is the identifier, not the name: /var/packages/dsm-mini, the upgrade
# path and every API call go by it, so it stays as it is. The visible name is
# displayname.
package="dsm-mini"
version="$SPK_VERSION"
os_min_ver="7.0-40000"
checksum="$PKG_CHECKSUM"
displayname="DSM mini (Telegram Mini App)"
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
# `--format=ustar` rather than whatever the local tar defaults to: BSD tar
# writes pax, and a pax header is one more entry in front of the real one.
( cd "$WORK" && tar cpf "$SPK" --format=ustar --owner=root --group=root \
    package.tgz INFO scripts PACKAGE_ICON.PNG PACKAGE_ICON_256.PNG WIZARD_UIFILES conf LICENSE )

# The archive is checked rather than trusted: the same tar that writes the
# AppleDouble entries also hides them when listing, so the only way to see what
# is really inside is to read it with something else. The first entry has to be
# package.tgz — that is where Package Center looks.
python3 - "$SPK" <<'PYEOF'
import sys, tarfile

names = tarfile.open(sys.argv[1]).getnames()
junk = [n for n in names if n.split("/")[-1].startswith("._") or "PaxHeader" in n]
if junk:
    sys.exit("%d macOS metadata entries in the package: %s" % (len(junk), junk[:3]))
if names[0] != "package.tgz":
    sys.exit("the package starts with %r, package.tgz expected" % names[0])
PYEOF

echo "done: $SPK ($(du -h "$SPK" | cut -f1))"
