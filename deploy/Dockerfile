# Сборка фронтенда: результат попадает в internal/web/dist, откуда его
# забирает go:embed на следующем шаге.
#
# --platform=$BUILDPLATFORM здесь и ниже важен для многоархитектурного образа:
# без него сборка под arm64 шла бы через эмуляцию и занимала бы десятки минут.
# Собираем всегда на своей архитектуре, а под чужую только кросс-компилируем.
FROM --platform=$BUILDPLATFORM node:22-alpine AS web
WORKDIR /src
COPY web/package.json web/package-lock.json* ./
RUN npm ci --no-audit --no-fund || npm install --no-audit --no-fund
COPY web/ ./
RUN npm run build

# Сборка бинарника со встроенным Mini App.
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
COPY --from=web /internal/web/dist ./internal/web/dist

ARG VERSION=dev
ARG TARGETOS
ARG TARGETARCH
# CGO не нужен: получается статический бинарник, которому не нужна libc
# в финальном образе. Он же позволяет кросс-компилировать без тулчейна.
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o /out/dsm-mini ./cmd/dsm-mini

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /
COPY --from=build /out/dsm-mini /dsm-mini
# Состояние наблюдателя переживает перезапуск, чтобы сервис не слал заново
# уведомления о задачах, завершившихся до него.
VOLUME ["/data"]
ENV STATE_DIR=/data
EXPOSE 8080
USER nonroot:nonroot
# Проверка здоровья — тем же бинарником: в distroless нет ни shell, ни curl.
HEALTHCHECK --interval=60s --timeout=5s --start-period=20s --retries=3 \
    CMD ["/dsm-mini", "-healthcheck"]
ENTRYPOINT ["/dsm-mini"]
