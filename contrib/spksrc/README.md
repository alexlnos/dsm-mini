# The SynoCommunity recipe

SynoCommunity is the community catalogue of Synology packages: its source is
already added by a great many people, and getting in there means getting "into
the store" without Synology's partner programme.

They build everything themselves, from source, with their own toolchain — so
our `tools/build-spk.sh` does not suit them and this recipe lives alongside it.

## What is here

```
cross/dsm-mini/     builds the binary: fetches our tag from GitHub and builds Go
spk/dsm-mini/       the package itself: description, install wizard, service start
```

The built Mini App interface lives in the repository and goes into the binary
through `go:embed`, so no Node is needed for the build — only Go. That makes
life on their build farm noticeably easier.

## Checking the build

```bash
git clone https://github.com/SynoCommunity/spksrc
cd spksrc
cp -r /path/to/dsm-mini/contrib/spksrc/cross/dsm-mini cross/
cp -r /path/to/dsm-mini/contrib/spksrc/spk/dsm-mini spk/
make -C spk/dsm-mini arch-x64-7.1
```

The first build pulls a toolchain and takes a while. The result lands in
`packages/`.

## Submitting

1. Fork [SynoCommunity/spksrc](https://github.com/SynoCommunity/spksrc).
2. Put both directories in place, as above.
3. Run the build for at least one architecture.
4. Open a pull request; their requirements are in the
   [documentation](https://docs.synocommunity.com/developer-guide/).

## On a new release

Update the version in both `Makefile` files (`PKG_VERS`, `SPK_VERS`), bump
`SPK_REV` and recompute the source checksums:

```bash
curl -sL https://github.com/alexlnos/dsm-mini/archive/refs/tags/vX.Y.Z.tar.gz -o s.tar.gz
for a in sha1 sha256 md5; do
  echo "dsm-mini-X.Y.Z.tar.gz ${a^^} $(openssl dgst -$a s.tar.gz | awk '{print $2}')"
done
```
