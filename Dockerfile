# syntax=docker/dockerfile:1

FROM golang:1.26 AS builder

WORKDIR /src

# Dependencies first, cached separately from app code so editing sources
# doesn't invalidate this layer.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY main.go ./
COPY internal ./internal

# Fully static binary so it runs on a scratch base.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server .


FROM scratch

# Root CAs for outbound HTTPS (core agent, Silpo token refresh). The app uses
# no time zones, so tzdata is deliberately left out — add `import _
# "time/tzdata"` to main.go if that ever changes.
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /out/server /server

ENV PORT=8080

# scratch has no /etc/passwd, so the non-root user is numeric (the same UID
# distroless calls "nonroot").
USER 65532:65532

EXPOSE 8080

# DATABASE_URL, JWT_SECRET, CORE_AGENT_URL, CORE_SERVICE_TOKEN and
# SILPO_REFRESH_URL are read from the environment at startup (see
# internal/config) — pass them with `docker run -e` / --env-file, never bake
# them into the image.
#
# Cloud Run injects PORT itself (usually 8080) and requires the container to
# listen on it; the app already reads $PORT from the environment, so no shell
# expansion is needed here.
ENTRYPOINT ["/server"]
