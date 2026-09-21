#!/usr/bin/env python3
"""Builds the screenshots for the Package Center listing.

The gallery in Package Center is wide, and our screenshots are a phone: put
one straight in and it is a narrow strip in the middle of an empty frame. So
each slide is composed — the screen on one side, a line about what is on it on
the other.

Package Center does not load these itself: it asks the NAS to fetch them and
streams the bytes back (SYNO.Core.Package.Screenshot.Server, checked on a live
NAS). So they have to be reachable from the NAS and in an ordinary format —
hence PNG rather than the WebP the READMEs use.

    tools/make-store.py

Result: docs/store/*.png, which tools/make-feed.py publishes and lists in the
catalogue under `snapshot`.
"""
import base64
import pathlib
import subprocess
import tempfile

ROOT = pathlib.Path(__file__).resolve().parent.parent
SHOTS = ROOT / "docs" / "screenshots"
OUT = ROOT / "docs" / "store"

W, H = 1280, 720

# The English screens: the catalogue is one file for everyone, and the
# `language` the Package Center asks with cannot reach a static file.
SLIDES = [
    ("downloads", "Downloads, from the chat",
     "Send a magnet link and the bot asks where to put it.",
     "Progress, speed, time left, seeds and peers — and pause, resume or delete on the spot."),
    ("task", "Everything about one task",
     "Open a download and see what is inside it.",
     "Pick which files of a torrent to fetch and in which order; trackers and peers are there too."),
    ("files", "The files themselves",
     "File Station without the web interface.",
     "Browse folders, preview, upload from the phone, rename, move and delete."),
    ("home", "How the NAS is doing",
     "The state of the machine on one screen.",
     "Processor and memory, uptime, network, and the DSM event log underneath."),
    ("storage", "Disks and volumes",
     "What the data actually sits on.",
     "Temperature and health of every disk, how full the pools and volumes are."),
]

INK, MUTED, ACCENT, BG, CARD = "#1C1E26", "#6E7180", "#2B44D6", "#F4F5FA", "#FFFFFF"

SLIDE = """<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink"
     width="{W}" height="{H}" viewBox="0 0 {W} {H}">
  <defs>
    <linearGradient id="bg" x1="0" y1="0" x2="1" y2="1">
      <stop offset="0" stop-color="{BG}"/><stop offset="1" stop-color="#E8EAF6"/>
    </linearGradient>
    <clipPath id="screen">
      <rect x="{sx}" y="{sy}" width="{sw}" height="{sh}" rx="26"/>
    </clipPath>
  </defs>
  <rect width="{W}" height="{H}" fill="url(#bg)"/>

  <rect x="88" y="196" width="56" height="6" rx="3" fill="{ACCENT}"/>
  <text x="88" y="268" font-family="Helvetica, Arial, sans-serif" font-size="52"
        font-weight="700" fill="{INK}">{title}</text>
  <text x="88" y="324" font-family="Helvetica, Arial, sans-serif" font-size="27"
        fill="{INK}">{lead}</text>
  <text x="88" y="384" font-family="Helvetica, Arial, sans-serif" font-size="22"
        fill="{MUTED}">{line1}</text>
  <text x="88" y="416" font-family="Helvetica, Arial, sans-serif" font-size="22"
        fill="{MUTED}">{line2}</text>

  <rect x="{fx}" y="{fy}" width="{fw}" height="{fh}" rx="34" fill="{CARD}"/>
  <image x="{sx}" y="{sy}" width="{sw}" height="{sh}" clip-path="url(#screen)"
         xlink:href="{href}"/>
</svg>
"""


def wrap(text: str, limit: int) -> tuple[str, str]:
    """Splits one sentence into two lines — librsvg has no text wrapping."""
    if len(text) <= limit:
        return text, ""
    cut = text.rfind(" ", 0, limit)
    return text[:cut], text[cut + 1:]


def escape(text: str) -> str:
    return text.replace("&", "&amp;").replace("<", "&lt;").replace(">", "&gt;")


def main() -> None:
    OUT.mkdir(parents=True, exist_ok=True)

    for index, (name, title, lead, detail) in enumerate(SLIDES, start=1):
        source = SHOTS / f"en-{name}.webp"
        if not source.is_file():
            raise SystemExit(f"no screenshot {source} — run tools/shoot.js first")

        with tempfile.TemporaryDirectory() as tmp:
            png = pathlib.Path(tmp) / "shot.png"
            # sips reads WebP; librsvg embeds PNG only.
            subprocess.run(["sips", "-s", "format", "png", str(source), "--out", str(png)],
                           check=True, capture_output=True)
            data = base64.b64encode(png.read_bytes()).decode()

        # The screen keeps its proportions, the frame is a little larger.
        sh = 560
        sw = round(sh * 520 / 1125)
        sx, sy = W - 200 - sw, (H - sh) // 2
        pad = 14

        line1, line2 = wrap(detail, 62)
        svg = SLIDE.format(
            W=W, H=H, BG=BG, CARD=CARD, INK=INK, MUTED=MUTED, ACCENT=ACCENT,
            title=escape(title), lead=escape(lead),
            line1=escape(line1), line2=escape(line2),
            sx=sx, sy=sy, sw=sw, sh=sh,
            fx=sx - pad, fy=sy - pad, fw=sw + pad * 2, fh=sh + pad * 2,
            href="data:image/png;base64," + data)

        with tempfile.NamedTemporaryFile("w", suffix=".svg", delete=False) as f:
            f.write(svg)
            tmp_svg = f.name
        target = OUT / f"{index:02d}-{name}.png"
        subprocess.run(["rsvg-convert", "-w", str(W), "-h", str(H), tmp_svg, "-o", str(target)],
                       check=True)
        pathlib.Path(tmp_svg).unlink()
        print(f"  {target.name}")

    print(f"{len(SLIDES)} slides in {OUT}")


if __name__ == "__main__":
    main()
