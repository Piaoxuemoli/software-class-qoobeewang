#!/usr/bin/env python3
"""Generate docs/templates/reference.docx (pandoc reference document).

Reproducible: starts from pandoc's built-in reference.docx, then adjusts it
with python-docx for Chinese-friendly output:
  - docDefaults / Normal: 12pt, Latin Times New Roman, East Asian SimSun
  - headings: SimHei (East Asian)
  - page: A4, normal margins (2.54cm top/bottom, 3.17cm left/right)
  - "Table" style: single-line borders

SimSun/SimHei are resolved to Noto CJK on Linux via docker/fonts-alias.conf.

Run inside the docs image, e.g.:
  docker run --rm -v "$PWD:/workspace" -w /workspace databackup-docs \\
      python3 scripts/docs/make-reference-docx.py docs/templates/reference.docx
"""

import subprocess
import sys
import tempfile
from pathlib import Path

import docx
from docx.oxml.ns import qn
from docx.shared import Cm, Pt

EAST_ASIA_BODY = "宋体"
EAST_ASIA_HEADING = "黑体"
LATIN = "Times New Roman"


def set_rfonts(rpr, *, ascii_=None, eastasia=None):
    """Set w:rFonts on an rPr element (create it if needed)."""
    rfonts = rpr.find(qn("w:rFonts"))
    if rfonts is None:
        rfonts = rpr.makeelement(qn("w:rFonts"), {})
        rpr.insert(0, rfonts)
    if ascii_:
        rfonts.set(qn("w:ascii"), ascii_)
        rfonts.set(qn("w:hAnsi"), ascii_)
        rfonts.set(qn("w:cs"), ascii_)
    if eastasia:
        rfonts.set(qn("w:eastAsia"), eastasia)


def style_rpr(style):
    """Return (creating if needed) the w:rPr of a style element."""
    rpr = style.element.find(qn("w:rPr"))
    if rpr is None:
        rpr = style.element.makeelement(qn("w:rPr"), {})
        style.element.append(rpr)
    return rpr


def set_color(style, hex_rgb):
    """Force a style's font color (pandoc's default headings are blue).

    A w:themeColor attribute overrides w:val, so theme attributes must be
    dropped, not just recolored.
    """
    rpr = style_rpr(style)
    color = rpr.find(qn("w:color"))
    if color is None:
        color = rpr.makeelement(qn("w:color"), {})
        rpr.append(color)
    color.set(qn("w:val"), hex_rgb)
    for attr in ("themeColor", "themeShade", "themeTint"):
        color.attrib.pop(qn(f"w:{attr}"), None)


def main() -> int:
    out = Path(sys.argv[1] if len(sys.argv) > 1 else "docs/templates/reference.docx")

    with tempfile.TemporaryDirectory() as td:
        base = Path(td) / "reference.docx"
        data = subprocess.run(
            ["pandoc", "--print-default-data-file", "reference.docx"],
            check=True,
            capture_output=True,
        ).stdout
        base.write_bytes(data)

        doc = docx.Document(str(base))

        # Global defaults: Latin Times New Roman, East Asian SimSun.
        styles_el = doc.styles.element
        docdefaults = styles_el.find(qn("w:docDefaults"))
        rprd = docdefaults.find(qn("w:rPrDefault"))
        rpr = rprd.find(qn("w:rPr"))
        if rpr is None:
            rpr = rprd.makeelement(qn("w:rPr"), {})
            rprd.append(rpr)
        set_rfonts(rpr, ascii_=LATIN, eastasia=EAST_ASIA_BODY)

        # Normal: 12pt body text.
        normal = doc.styles["Normal"]
        normal.font.size = Pt(12)
        set_rfonts(style_rpr(normal), ascii_=LATIN, eastasia=EAST_ASIA_BODY)

        # Headings: SimHei for CJK, black (pandoc's default is blue).
        # The linked character styles (Heading N Char) carry the theme color
        # too — fix both so whichever wins, the result is black.
        for name in (
            ["Title", "Title Char", "Subtitle", "Subtitle Char", "TOC Heading"]
            + [f"Heading {i}" for i in range(1, 10)]
            + [f"Heading {i} Char" for i in range(1, 10)]
        ):
            try:
                st = doc.styles[name]
            except KeyError:
                continue
            set_rfonts(style_rpr(st), ascii_=LATIN, eastasia=EAST_ASIA_HEADING)
            set_color(st, "000000")

        # A4, normal margins.
        for sec in doc.sections:
            sec.page_width = Cm(21.0)
            sec.page_height = Cm(29.7)
            sec.top_margin = Cm(2.54)
            sec.bottom_margin = Cm(2.54)
            sec.left_margin = Cm(3.17)
            sec.right_margin = Cm(3.17)

        # Table style: single-line borders on all sides + inside.
        try:
            tbl = doc.styles["Table"]
        except KeyError:
            tbl = None
        if tbl is not None:
            tblpr = tbl.element.find(qn("w:tblPr"))
            if tblpr is None:
                tblpr = tbl.element.makeelement(qn("w:tblPr"), {})
                tbl.element.append(tblpr)
            borders = tblpr.makeelement(qn("w:tblBorders"), {})
            for edge in ("top", "left", "bottom", "right", "insideH", "insideV"):
                el = borders.makeelement(qn(f"w:{edge}"), {})
                el.set(qn("w:val"), "single")
                el.set(qn("w:sz"), "4")
                el.set(qn("w:space"), "0")
                el.set(qn("w:color"), "auto")
                borders.append(el)
            tblpr.append(borders)
        else:
            print("warning: 'Table' style not found in reference doc", file=sys.stderr)

        out.parent.mkdir(parents=True, exist_ok=True)
        doc.save(str(out))
    print(f"wrote {out}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
