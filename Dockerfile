FROM node:24-alpine AS web-deps
WORKDIR /src/web
COPY web/package*.json ./
RUN npm ci && sha256sum package-lock.json | cut -d ' ' -f 1 > node_modules/.clip-share-package-lock.sha256

FROM web-deps AS web-build
COPY web/ ./
RUN npm run build

FROM golang:1.25-alpine AS go-build
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
COPY --from=web-build /src/web/dist/ ./internal/webui/dist/
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/clip-share ./cmd/server

FROM golang:1.25-alpine AS api-dev
RUN apk add --no-cache ffmpeg && go install github.com/air-verse/air@v1.63.0
WORKDIR /src
CMD ["air", "-c", ".air.toml"]

FROM web-deps AS web-dev
COPY web/ ./
CMD ["sh", "/src/web/docker-entrypoint.sh"]

FROM alpine:3.22 AS production
RUN apk add --no-cache ca-certificates ffmpeg wget && addgroup -S clipshare && adduser -S -G clipshare clipshare && mkdir -p /data && chown clipshare:clipshare /data
COPY --from=go-build /out/clip-share /usr/local/bin/clip-share
USER clipshare
EXPOSE 8080
VOLUME ["/data"]
ENTRYPOINT ["/usr/local/bin/clip-share"]
