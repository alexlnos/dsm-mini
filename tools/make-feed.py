#!/usr/bin/env python3
"""Builds the catalogue for the DSM Package Center "Package Sources".

The Package Center calls the given address with the `arch`, `build` and
`language` parameters and expects JSON with a list of matching packages. The
server is supposed to filter that list itself — which is why big catalogues
(SynoCommunity, 007revad) run a live handler there.

We have almost nothing to filter: one package, two architectures. So instead
of a handler we lay out one static file per architecture — GitHub Pages serves
them over an ordinary GET and no server is needed. People add the address
matching their model.

The catalogue has to carry the md5 and the size of every .spk: the Package
Center reads both out of it and passes them to the backend as `checksum` and
`filesize` (see PkgManApp.js on the NAS, the SYNO.Core.Package.Installation
upgrade call). Without them an update ends with "Invalid file format".

    tools/make-feed.py 1.2.3 --spk dist --out public
"""
import argparse
import hashlib
import json
import pathlib
import shutil
import urllib.request

REPO = "alexlnos/dsm-mini"
PAGES = f"https://{REPO.split('/')[0]}.github.io/{REPO.split('/')[1]}"

# The same families as in build-spk.sh: that is how DSM reports itself.
ARCHS = {
    "amd64": ["x86_64", "apollolake", "avoton", "braswell", "broadwell", "broadwellnk",
              "broadwellnkv2", "broadwellntbap", "bromolow", "cedarview", "denverton",
              "epyc7002", "epyc7003", "epyc7003ntb", "geminilake", "geminilakenk",
              "grantley", "kvmx64", "purley", "r1000", "r1000nk", "v1000", "v1000nk"],
    "arm64": ["aarch64", "rtd1296", "rtd1619b", "armada37xx"],
}

# The listing text lives in assets/store.json together with the one the
# package itself carries: two copies of the same paragraph drift apart.
STORE = json.loads((pathlib.Path(__file__).resolve().parent.parent
                    / "assets" / "store.json").read_text(encoding="utf-8"))
DESC = STORE["description"]["enu"]

# Package Center does not fetch these itself — it asks the NAS to, and streams
# the bytes back. So they are ordinary PNGs on the same site as the catalogue.
SHOTS = sorted((pathlib.Path(__file__).resolve().parent.parent
                / "docs" / "store").glob("*.png"))


# The build number at the end of a Synology version. Pinned at 0 and kept in
# step with tools/build-spk.sh: Package Center compares the catalogue's version
# against the one in INFO, so the two have to be spelled the same way. The file
# name and the tag stay plain — nothing compares those.
BUILD = 0


def entry(version: str, arch: str, spk_dir: pathlib.Path) -> dict:
    spk = f"dsm-mini-{version}-{arch}.spk"

    # The file itself is the source of the checksum and the size: taking them
    # from anywhere else is how a catalogue starts describing a package that
    # no longer exists.
    path = spk_dir / spk
    if not path.is_file():
        raise SystemExit(f"no package {path} — build it before the catalogue")
    body = path.read_bytes()

    return {
        "package": "dsm-mini",
        "version": f"{version}-{BUILD}",
        "dname": "DSM mini (Telegram Mini App)",
        "desc": DESC,
        # The release asset, and the redirect it answers with is fine — it was
        # never the problem. What refused the package was its own contents:
        # macOS tar had been writing AppleDouble entries into the archive (see
        # tools/build-spk.sh). The link lives here rather than on Pages for a
        # plainer reason — Pages compresses what it serves, and a package that
        # arrives gzip-encoded is one more thing that can go wrong for no gain:
        #
        #   Pages:   content-encoding: gzip, content-length: 5249303
        #   release: content-length: 5335040, served as it is
        "link": f"https://github.com/{REPO}/releases/download/v{version.split('-')[0]}/{spk}",
        "md5": hashlib.md5(body).hexdigest(),
        "size": len(body),
        "thumbnail": [f"{PAGES}/icon_72.png"],
        "thumbnail_retina": [f"{PAGES}/icon_256.png"],
        "maintainer": "alexlnos",
        "maintainer_url": f"https://github.com/{REPO}",
        # Package Center shows this as text under "What is new", not as a
        # link: a URL here renders as a URL, which is what it used to do.
        "changelog": STORE["changelog"],
        "snapshot": [f"{PAGES}/shots/{p.name}" for p in SHOTS],
        "distributor": "alexlnos",
        "distributor_url": f"https://github.com/{REPO}",
        "support_url": f"https://github.com/{REPO}/issues",
        "price": 0,
        "beta": False,
        # Installing without the wizard is impossible: without a bot token the service will not start.
        "qinst": False,
        "qupgrade": True,
        "qstart": False,
        "start": True,
    }


INDEX = """<!doctype html>
<html lang="en">
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>dsm-mini — package source</title>
<style>
  body {{ max-width: 44rem; margin: 3rem auto; padding: 0 1.25rem;
         font: 16px/1.6 -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
         color: #1c1c1e; }}
  code, pre {{ background: #f2f2f7; border-radius: 6px; }}
  code {{ padding: 2px 6px; }}
  pre {{ padding: 12px 14px; overflow-x: auto; }}
  h1 {{ display: flex; align-items: center; gap: .6rem; }}
  img {{ width: 48px; height: 48px; }}
  table {{ border-collapse: collapse; width: 100%; }}
  td, th {{ text-align: left; padding: 6px 10px 6px 0; border-bottom: 1px solid #e5e5ea; }}
  a {{ color: #0067d8; }}
</style>
<h1><img src="icon_256.png" alt=""> dsm-mini</h1>
<p>Telegram bot with a Mini App to manage your Synology NAS.
   <a href="https://github.com/{repo}">Source code</a>.</p>

<h2>Add to Package Center</h2>
<p><b>Package Center → Settings → Package Sources → Add</b>, then paste the
   address for your NAS architecture:</p>
<table>
  <tr><th>Your NAS</th><th>Address</th></tr>
  <tr><td>Intel / AMD (most models)</td><td><code>{pages}/amd64.json</code></td></tr>
  <tr><td>ARM (budget models)</td><td><code>{pages}/arm64.json</code></td></tr>
</table>
<p>Not sure which one? Package Center refuses a package built for another
   architecture, so a wrong guess costs nothing.</p>
<p>The packages themselves live in
   <a href="https://github.com/{repo}/releases">GitHub Releases</a>; this page
   only tells Package Center where to find them.</p>
<p>Current version: <b>{version}</b></p>
"""


def verify(link: str, md5: str, size: int) -> None:
    """Check that the link really serves the bytes the catalogue promises.

    It did not, once. The link pointed at GitHub Pages, which compresses what
    it serves: a client that says it understands gzip gets 5 249 303 bytes of
    gzip instead of the 5 335 040 of the package, and no md5 in the catalogue
    can describe both. Package Center says "invalid file format" and names
    nothing, so the check is done here, where the two values are still side by
    side. The request asks for gzip on purpose — the point is to see what a
    downloader that accepts it would be handed.
    """
    request = urllib.request.Request(link, headers={"Accept-Encoding": "gzip"})
    with urllib.request.urlopen(request, timeout=120) as response:
        encoding = response.headers.get("Content-Encoding", "")
        body = response.read()  # raw: urllib does not decode gzip by itself

    if encoding:
        raise SystemExit(
            f"{link}\n  served with Content-Encoding: {encoding} — the package "
            f"has to arrive as it is, or its checksum describes nothing")
    if len(body) != size:
        raise SystemExit(f"{link}\n  {len(body)} bytes, {size} in the catalogue")
    got = hashlib.md5(body).hexdigest()
    if got != md5:
        raise SystemExit(f"{link}\n  md5 {got}, {md5} in the catalogue")
    print(f"  {link.rsplit('/', 1)[-1]}: {size} bytes, md5 matches")


def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument("version", help="package version, for example 1.2.3-1")
    ap.add_argument("--spk", default="dist", help="directory with the built .spk files")
    ap.add_argument("--out", default="public", help="where to put the catalogue")
    ap.add_argument("--verify", action="store_true",
                    help="download every link and check it against the catalogue")
    args = ap.parse_args()

    out = pathlib.Path(args.out)
    out.mkdir(parents=True, exist_ok=True)

    spk_dir = pathlib.Path(args.spk)
    for arch, syno_archs in ARCHS.items():
        package = entry(args.version, arch, spk_dir)
        if args.verify:
            verify(package["link"], package["md5"], package["size"])
        body = {"packages": [package]}
        (out / f"{arch}.json").write_text(json.dumps(body, indent=2) + "\n", encoding="utf-8")
        # Every platform name gets its own file: people should not have to
        # know their epyc7002 is amd64. The files are identical, so it is a copy.
        for syno in syno_archs:
            (out / f"{syno}.json").write_text(json.dumps(body, indent=2) + "\n", encoding="utf-8")

    shots = out / "shots"
    shots.mkdir(exist_ok=True)
    for shot in SHOTS:
        shutil.copy2(shot, shots / shot.name)


    (out / "index.html").write_text(
        INDEX.format(repo=REPO, pages=PAGES, version=args.version), encoding="utf-8")

    print(f"catalogue ready: {len(list(out.glob('*.json')))} files, "
          f"{len(SHOTS)} screenshots in {out}")


if __name__ == "__main__":
    main()
