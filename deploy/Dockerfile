# Сборка фронтенда: результат попадает в internal/web/dist, откуда его
# забирает go:embed на следующем шаге.
FROM node:22-alpine AS web
WORKDIR /src
COPY web/package.json web/package-lock.json* ./
RUN npm ci --no-audit --no-fund || npm install --no-audit --no-fund
COPY web/ ./
RUN npm run build

# Сборка бинарника со встроенным Mini App.
FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
COPY --from=web /internal/web/dist ./internal/web/dist
ARG VERSION=dev
# CGO не нужен: получается статический бинарник, которому не нужна libc
# в финальном образе.
RUN CGO_ENABLED=0 go build -trimpath \
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
ENTRYPOINT ["/dsm-mini"]
