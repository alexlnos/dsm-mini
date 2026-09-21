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

DESC = ("Telegram bot with a Mini App to manage your Synology NAS: "
        "Download Station, File Station, disks, virtual machines and containers.")


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
        "version": version,
        "dname": "dsm-mini",
        "desc": DESC,
        # The file lives in a GitHub release: the catalogue only names it.
        "link": f"https://github.com/{REPO}/releases/download/v{version.split('-')[0]}/{spk}",
        "md5": hashlib.md5(body).hexdigest(),
        "size": len(body),
        "thumbnail": [f"{PAGES}/icon_72.png"],
        "thumbnail_retina": [f"{PAGES}/icon_256.png"],
        "maintainer": "alexlnos",
        "maintainer_url": f"https://github.com/{REPO}",
        "changelog": f"https://github.com/{REPO}/releases",
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


def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument("version", help="package version, for example 1.2.3-1")
    ap.add_argument("--spk", default="dist", help="directory with the built .spk files")
    ap.add_argument("--out", default="public", help="where to put the catalogue")
    args = ap.parse_args()

    out = pathlib.Path(args.out)
    out.mkdir(parents=True, exist_ok=True)

    spk_dir = pathlib.Path(args.spk)
    for arch, syno_archs in ARCHS.items():
        body = {"packages": [entry(args.version, arch, spk_dir)]}
        (out / f"{arch}.json").write_text(json.dumps(body, indent=2) + "\n", encoding="utf-8")
        # Every platform name gets its own file: people should not have to
        # know their epyc7002 is amd64. The files are identical, so it is a copy.
        for syno in syno_archs:
            (out / f"{syno}.json").write_text(json.dumps(body, indent=2) + "\n", encoding="utf-8")

    (out / "index.html").write_text(
        INDEX.format(repo=REPO, pages=PAGES, version=args.version), encoding="utf-8")

    print(f"catalogue ready: {len(list(out.glob('*.json')))} files in {out}")


if __name__ == "__main__":
    main()
