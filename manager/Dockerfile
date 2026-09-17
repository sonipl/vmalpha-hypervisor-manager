# ── Stage 1: Build Go API ──
FROM golang:1.27.1-alpine AS api-builder
ENV GOTOOLCHAIN=local

# CGO disabled below — skip gcc/musl-dev (needs Alpine CDN)
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o novasphere-api ./cmd/novasphere-api

# ── Stage 2: Build React Frontend ──
FROM node:24-alpine AS web-builder

WORKDIR /build
COPY web/package.json web/package-lock.json* ./
RUN npm ci --no-audit

COPY web/ .
RUN npm run typecheck && npm run build

# ── Stage 3: Runtime ──
FROM alpine:3.24
ARG http_proxy
ARG https_proxy
ENV http_proxy=$http_proxy https_proxy=$https_proxy
RUN apk add --no-cache ca-certificates tzdata util-linux

RUN addgroup -S nova && adduser -S nova -G nova

WORKDIR /app

COPY --from=api-builder /build/novasphere-api .
COPY --from=web-builder /build/dist ./web/dist

# Supply configuration at runtime through /etc/novasphere/config.yaml.
# Never bake deployment credentials into this image.

RUN chown -R nova:nova /app
USER nova

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s \
  CMD wget -qO- http://localhost:8080/api/v1/health || exit 1

ENTRYPOINT ["./novasphere-api"]
