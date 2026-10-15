# Dev/CI toolchain image. Build from repo root:
#   docker build -f docker/dev.Dockerfile -t databackup-dev .
FROM ubuntu:22.04

# Optional Go module proxy for networks where proxy.golang.org is unreachable
# (empty = Go default). Passed as --build-arg by scripts/docker.sh.
ARG GOPROXY=""
ENV GOPROXY=${GOPROXY}

COPY scripts/install-deps.sh /tmp/install-deps.sh
RUN bash /tmp/install-deps.sh && rm -rf /var/lib/apt/lists/*

WORKDIR /workspace
