# Frontend build: the result lands in internal/web/dist, where go:embed picks
# it up on the next step.
#
# --platform=$BUILDPLATFORM here and below matters for a multi-arch image:
# without it an arm64 build would go through emulation and take tens of
# minutes. We always build on our own architecture and only cross-compile.
FROM --platform=$BUILDPLATFORM node:22-alpine AS web
WORKDIR /src
COPY web/package.json web/package-lock.json* ./
RUN npm ci --no-audit --no-fund || npm install --no-audit --no-fund
COPY web/ ./
RUN npm run build

# Building the binary with the Mini App embedded.
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
COPY --from=web /internal/web/dist ./internal/web/dist

ARG VERSION=dev
ARG TARGETOS
ARG TARGETARCH
# CGO is not needed: the result is a static binary that needs no libc in the
# final image. It also allows cross-compiling without a toolchain.
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o /out/dsm-mini ./cmd/dsm-mini

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /
COPY --from=build /out/dsm-mini /dsm-mini
# The watcher state survives a restart so the service does not resend
# notifications about tasks that finished before it.
VOLUME ["/data"]
ENV STATE_DIR=/data
EXPOSE 8080
USER nonroot:nonroot
# The health check uses the same binary: distroless has neither shell nor curl.
HEALTHCHECK --interval=60s --timeout=5s --start-period=20s --retries=3 \
    CMD ["/dsm-mini", "-healthcheck"]
ENTRYPOINT ["/dsm-mini"]
