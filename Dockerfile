# BuildKit sets BUILDPLATFORM; the legacy builder takes it as a build arg (just docker-build)
ARG BUILDPLATFORM=linux/amd64

# build stages run on the builder's own platform and cross-compile for the target
FROM --platform=$BUILDPLATFORM node:24-alpine AS web
WORKDIR /app/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS backend
ARG TARGETOS=linux
ARG TARGETARCH=amd64
WORKDIR /build
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /prulights ./cmd/prulights

FROM alpine:3.24
RUN apk --no-cache add ca-certificates tzdata && \
    adduser -S -D -H -h /app appuser && \
    mkdir -p /data && chown appuser /data
USER appuser
WORKDIR /app
COPY --from=backend /prulights ./prulights
COPY --from=web /app/web/out ./out
ENV STATIC_DIR=/app/out \
    DATABASE_PATH=/data/prulights.db \
    PORT=8080
VOLUME /data
EXPOSE 8080
ENTRYPOINT ["./prulights"]
CMD ["serve"]
