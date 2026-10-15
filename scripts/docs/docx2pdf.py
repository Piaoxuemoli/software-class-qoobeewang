#!/usr/bin/env python3
"""docx2pdf.py IN.docx OUT.pdf

Convert a .docx to PDF via LibreOffice's UNO API, refreshing all document
indexes (TOC) first — `soffice --convert-to pdf` does NOT update fields, so a
pandoc-generated TOC would export empty.

Requires python3-uno (docs image). Spawns its own headless soffice with a
per-process user profile, connects over a socket, updates DocumentIndexes
(twice — page numbers settle after the first pass), exports writer_pdf_Export.
"""

import os
import subprocess
import sys
import time

import uno
from com.sun.star.beans import PropertyValue


def prop(name, value):
    p = PropertyValue()
    p.Name = name
    p.Value = value
    return p


def main() -> int:
    src, dst = sys.argv[1], sys.argv[2]
    src_url = "file://" + os.path.abspath(src)
    dst_url = "file://" + os.path.abspath(dst)

    port = 2400 + (os.getpid() % 1000)
    profile = f"/tmp/lo-uno-{os.getpid()}"
    proc = subprocess.Popen(
        [
            "soffice",
            "--headless",
            "--norestore",
            "--nologo",
            "--nolockcheck",
            f"-env:UserInstallation=file://{profile}",
            f"--accept=socket,host=127.0.0.1,port={port};urp;",
        ]
    )
    try:
        local_ctx = uno.getComponentContext()
        resolver = local_ctx.ServiceManager.createInstanceWithContext(
            "com.sun.star.bridge.UnoUrlResolver", local_ctx
        )
        ctx = None
        for _ in range(80):
            try:
                ctx = resolver.resolve(
                    "uno:socket,host=127.0.0.1,port=%d;urp;StarOffice.ComponentContext"
                    % port
                )
                break
            except Exception:
                time.sleep(0.5)
        if ctx is None:
            raise RuntimeError("cannot connect to headless soffice")

        desktop = ctx.ServiceManager.createInstanceWithContext(
            "com.sun.star.frame.Desktop", ctx
        )
        doc = desktop.loadComponentFromURL(
            src_url, "_blank", 0, (prop("Hidden", True),)
        )
        if doc is None:
            raise RuntimeError(f"LibreOffice failed to load {src}")
        try:
            for _ in range(2):
                indexes = doc.getDocumentIndexes()
                for i in range(indexes.getCount()):
                    indexes.getByIndex(i).update()
                doc.refresh()
            doc.storeToURL(dst_url, (prop("FilterName", "writer_pdf_Export"),))
        finally:
            doc.close(False)
    finally:
        proc.terminate()
        try:
            proc.wait(10)
        except subprocess.TimeoutExpired:
            proc.kill()

    if not os.path.isfile(dst):
        raise RuntimeError(f"no PDF produced at {dst}")
    print(f"docx2pdf: {src} -> {dst}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
