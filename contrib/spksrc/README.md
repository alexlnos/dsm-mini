# The SynoCommunity recipe

SynoCommunity is the community catalogue of Synology packages: its source is
already added by a great many people, and getting in there means getting "into
the store" without Synology's partner programme.

This recipe is written the way their packages are, not the way ours is: their
generic installer and service script, their templated wizard, their package
user `sc-dsm-mini`. It follows `spk/ddns-go` (DSM 7, Go, MIT) for the build and
`spk/gentoo-chroot` for the window in the DSM main menu. It was built with
their toolchain in their container before it was committed here.

## What is here

```
cross/dsm-mini/        the binary: fetches our tag from GitHub and builds it with Go
  PLIST                what the cross build hands over — the binary
  digests              checksums of the source tarball, from `make digests`
spk/dsm-mini/          the package
  src/service-setup.sh settings file on install, environment on start
  src/wizard_templates/ one JSON template, one .yml per language
  src/app/             the settings window in the DSM main menu
```

The built Mini App lives in the repository and goes into the binary through
`go:embed`, so the build needs Go and nothing else.

## Where it differs from our own package, and why

- **Port 58080, not 8080.** In the SynoCommunity port list 8080 belongs to
  SABnzbd — a download package, installed by exactly the kind of people who
  install this one. Their rule for internal services is the 49152–65535 range.
- **No `SERVICE_PORT`.** In their framework it generates a firewall rule and a
  desktop shortcut to the port. The service listens on loopback only, so the
  rule would open nothing useful and the shortcut would point at an address no
  browser can reach. The window comes from `DSM_UI_CONFIG` instead.
- **The log is kept across restarts** (`SVC_KEEP_LOG`). Their script starts it
  afresh by default, and a refusal logged just before an upgrade is often the
  only trace of why the upgrade was needed.
- **The display name has a dash, not brackets.** Their INFO step writes the
  name unquoted into a shell command, so brackets are a syntax error there, and
  quoted into `jq` elsewhere, so escaping them leaks a backslash. No package of
  theirs has brackets in its name; ours now reads "DSM mini — Telegram Mini App"
  everywhere.
- **Apostrophes in descriptions are written `\'`** — their convention, needed
  because the descriptions pass through the shell on the way into INFO.

The package user is `sc-dsm-mini`, where our own package runs as `dsm-mini`.
The two builds are therefore **not interchangeable on one NAS**: moving from
one to the other is seen by DSM as an ordinary upgrade, and the new service
cannot read the settings or the database, both mode 0600 and owned by the
other user. Remove one before installing the other.

## Building it

Their container, on macOS with the two flags their documentation requires:

```bash
git clone https://github.com/SynoCommunity/spksrc && cd spksrc
cp -r /path/to/dsm-mini/contrib/spksrc/cross/dsm-mini cross/
cp -r /path/to/dsm-mini/contrib/spksrc/spk/dsm-mini spk/
docker run --rm -it --platform=linux/amd64 -v "$(pwd)":/spksrc -w /spksrc \
  -e TAR_CMD="fakeroot tar" ghcr.io/synocommunity/spksrc /bin/bash
make setup
make -C cross/dsm-mini digests
make -C spk/dsm-mini ARCH=x64 TCVERSION=7.2
```

The result lands in `packages/`. The first build pulls the toolchain and, on an
Apple Silicon Mac, runs under emulation — expect a while.

If `maintainer=""` turns up in the INFO, the build could not reach the GitHub
API at that moment: the framework asks it for the maintainer's name. Remove the
INFO in `spk/dsm-mini/work-*` and build again.

## On a new release

Update `PKG_VERS` in `cross/dsm-mini/Makefile` and `SPK_VERS` in
`spk/dsm-mini/Makefile`, **increment `SPK_REV`** — it only ever grows, even when
the version changes — and regenerate the checksums with `make digests`.
