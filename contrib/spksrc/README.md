# The SynoCommunity recipe

SynoCommunity is the community catalogue of Synology packages: its source is
already added by a great many people, and getting in there means getting "into
the store" without Synology's partner programme.

This recipe is written the way their packages are, not the way ours is: their
generic installer and service script, their package user `sc-dsm-mini`. It
follows `spk/ddns-go` (DSM 7, Go, MIT) for the build and `spk/gentoo-chroot`
for the window in the DSM main menu. It was built with their toolchain in their
container before it was committed here.

## What is here

```
cross/dsm-mini/        the binary: fetches our tag from GitHub and builds it with Go
  PLIST                what the cross build hands over — the binary
  digests              checksums of the source tarball, from `make digests`
spk/dsm-mini/          the package
  src/service-setup.sh where the service keeps its files, and the webhook on uninstall
  src/app/             the settings window in the DSM main menu
```

The built Mini App lives in the repository and goes into the binary through
`go:embed`, so the build needs Go and nothing else.

There is no install wizard. The package starts with no settings and is set up
in its window in the DSM main menu, which is also where they are changed later;
the service reads them itself and applies a save without a restart.

`src/app/` holds copies of `spk/ui/` from the repository — `index.html`,
`app.js`, `i18n.js`, `api.cgi` and the images — and CI fails if they differ.
Only the Ext JS wrapper is the recipe's own, `dsm-mini.js`, because the
namespace of a DSM app is `SYNOCOMMUNITY.*` in their catalogue.

## One package, two builds

This recipe and the package this repository builds are the same package as far
as DSM is concerned: the same identifier, the same package user `sc-dsm-mini`
(their framework names it, and ours names it the same on purpose), the same
port 58080. So a NAS can move from one to the other as an ordinary upgrade and
keep its settings — which is how the repository's own builds serve as betas for
the releases published here.

It was not always so. When the repository's package ran as `dsm-mini`, an
upgrade to it from this build left a live NAS stopped with a Repair button:
DSM hands the data folder to the new package user on an upgrade, but not the
files inside it, and the new service could not read one of them.

Checked on a live NAS (SA6400, DSM 7.4) with 1.0.13 on 25 September 2026: the
repository's 1.0.13-0 was replaced by this build's 1.0.13-1 as an upgrade,
with the settings and the database kept; an uninstall with "Uninstall only"
removed the notification webhook and kept the files, and the next install
started on the kept settings and registered the webhook again.

## Where it differs from the repository's package, and why

- **No `SERVICE_PORT`.** In their framework it generates a firewall rule and a
  desktop shortcut to the port. The service listens on loopback only, so the
  rule would open nothing useful and the shortcut would point at an address no
  browser can reach. The window comes from `DSM_UI_CONFIG` instead.
- **The log is kept across restarts** (`SVC_KEEP_LOG`). Their script starts it
  afresh by default, and a refusal logged just before an upgrade is often the
  only trace of why the upgrade was needed. The repository's package appends to
  it as well.
- **The display name has a dash, not brackets.** Their INFO step writes the
  name unquoted into a shell command, so brackets are a syntax error there, and
  quoted into `jq` elsewhere, so escaping them leaks a backslash. No package of
  theirs has brackets in its name; ours now reads "DSM mini — Telegram Mini App"
  everywhere.
- **Apostrophes in descriptions are written `\'`** — their convention, needed
  because the descriptions pass through the shell on the way into INFO.

## Building it

One command from the repository, with Docker running:

```bash
tools/build-spksrc.sh                # x64, DSM 7.2
tools/build-spksrc.sh x64 aarch64    # their checklist asks for both
tools/build-spksrc.sh digests        # after a version change, back into cross/dsm-mini/digests
```

The script keeps a clone of their framework as a cache in
`~/Library/Caches/dsm-mini/spksrc` — over 3 GB with the toolchains and the
source tarballs, somebody else's repository and no part of this one — copies
this recipe into it afresh on every run, builds in their container with the two
flags their documentation requires on macOS (`--platform=linux/amd64`,
`TAR_CMD="fakeroot tar"`) and puts the package into `dist/`. The first run
clones and pulls the toolchain and, on an Apple Silicon Mac, runs under
emulation — expect a while. Their master moves on, so pull that clone before a
build meant for a pull request to them.

If `maintainer=""` turns up in the INFO, the build could not reach the GitHub
API at that moment: the framework asks it for the maintainer's name. Build
again.

## Go newer than their toolchain

Our `go.mod` asks for Go 1.27.1; spksrc ships 1.26.8 as `native/go`. The build
still works — the framework does not set `GOTOOLCHAIN`, so Go fetches the
newer version itself (`go: downloading go1.27.1 (linux/amd64)` in the build
log) — but it depends on the build being able to reach the internet. Their CI
can; worth saying in the pull request in case a reviewer would rather the
package did not outrun their toolchain.

## On a new release

Update `PKG_VERS` in `cross/dsm-mini/Makefile` and `SPK_VERS` in
`spk/dsm-mini/Makefile`, **increment `SPK_REV`** once the package is published
there — it only ever grows, even when the version changes — and regenerate the
checksums with `tools/build-spksrc.sh digests`. Copy
`spk/ui/` over `src/app/` if the window changed; CI says so when it has.
