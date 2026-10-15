# Docs toolchain image: pandoc + LibreOffice + PlantUML + CJK fonts.
# Build:  docker build -f docker/docs.Dockerfile -t databackup-docs .
FROM ubuntu:22.04

ARG PANDOC_VERSION=3.12
ARG PANDOC_SHA256=91903ff19f1b1d4db4129797c7e18f71212990d7394fcff1787719aebf04e372
ARG PLANTUML_VERSION=1.2026.8
ARG PLANTUML_SHA256=5e1ecfa8ecd32c90b03bbf3b1eb6f020943f98ab0fcf4032be31a0002ee2c462
# Optional transport fallbacks for github.com assets (see docker/gh-download.sh):
#   --build-arg GH_MIRROR=https://gh-proxy.com/   (URL prefix mirror)
#   --build-arg GH_PROXY=http://proxy:7890        (http(s) proxy)
ARG GH_MIRROR=https://gh-proxy.com/
ARG GH_PROXY=

RUN apt-get update \
 && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
      ca-certificates curl xz-utils git \
      libreoffice-writer fonts-noto-cjk fontconfig \
      graphviz default-jre-headless \
      python3-docx python3-uno poppler-utils \
 && rm -rf /var/lib/apt/lists/*

# Download helper for GitHub release assets: tries each URL directly, then via
# the optional GH_PROXY proxy and GH_MIRROR prefix. SHA256 is verified
# regardless of the route.
COPY docker/gh-download.sh /usr/local/bin/gh-download
RUN chmod +x /usr/local/bin/gh-download

# Pandoc: pinned .deb from GitHub releases, SHA256-verified.
RUN gh-download "/tmp/pandoc.deb" "${PANDOC_SHA256}" \
      "https://github.com/jgm/pandoc/releases/download/${PANDOC_VERSION}/pandoc-${PANDOC_VERSION}-1-amd64.deb" \
 && dpkg -i /tmp/pandoc.deb \
 && rm /tmp/pandoc.deb

# PlantUML: pinned jar (newer than the apt build), SHA256-verified,
# exposed as `plantuml`. Fallback mirror: Maven Central.
RUN gh-download "/opt/plantuml.jar" "${PLANTUML_SHA256}" \
      "https://github.com/plantuml/plantuml/releases/download/v${PLANTUML_VERSION}/plantuml-${PLANTUML_VERSION}.jar" \
      "https://repo1.maven.org/maven2/net/sourceforge/plantuml/plantuml/${PLANTUML_VERSION}/plantuml-${PLANTUML_VERSION}.jar" \
 && printf '#!/bin/sh\nexec java -jar /opt/plantuml.jar "$@"\n' > /usr/local/bin/plantuml \
 && chmod +x /usr/local/bin/plantuml

# Map the Chinese font names used in documents/styles onto Noto CJK.
COPY docker/fonts-alias.conf /etc/fonts/conf.d/99-cjk-alias.conf
RUN fc-cache -f >/dev/null

WORKDIR /workspace
