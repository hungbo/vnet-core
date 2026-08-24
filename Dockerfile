# VNET Core — one image serving both the API and the admin UI.
#
# The admin is built first and copied into the Go build context so it can be
# embedded into the binary, which is why deployment needs a single container
# and no web server in front.

# ---------------------------------------------------------------- admin build
FROM node:20-alpine AS admin

# pnpm 10: admin/pnpm-lock.yaml is lockfileVersion 9.0 and pnpm-workspace.yaml
# uses the pnpm 10 build-approval keys, so older majors refuse the lockfile.
RUN npm install -g pnpm@10

WORKDIR /admin

# Manifests first so the dependency layer is reused whenever only source
# changes. The workspace packages are needed here too: package.json references
# them through the workspace: protocol, so install fails without them.
COPY admin/package.json admin/pnpm-lock.yaml admin/pnpm-workspace.yaml ./
COPY admin/packages ./packages
RUN pnpm install --frozen-lockfile

COPY admin/ ./

# .env carries VITE_AUTH_ROUTE_MODE, VITE_ROUTE_HOME and the 8888/7777/9999
# response codes the backend contract depends on; .env.prod points the client
# at /api on the same origin. A build without them produces a broken admin.
RUN test -f .env || (echo "ERROR: admin/.env is missing — the admin cannot be built without it" && exit 1)
RUN pnpm build

# --------------------------------------------------------------- server build
FROM golang:1.25-alpine AS server

WORKDIR /src

COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ ./

# Replace the placeholder page with the real admin build before compiling.
COPY --from=admin /admin/dist/ ./cmd/server/embed/

ARG VERSION=docker
RUN CGO_ENABLED=0 GOOS=linux go build \
      -buildvcs=false \
      -ldflags="-s -w -X main.version=${VERSION}" \
      -o /out/vnet-server ./cmd/server

# The maintenance commands ship in the same image: they share this codebase and
# adding them costs a few megabytes instead of a second build pipeline.
RUN CGO_ENABLED=0 GOOS=linux go build -buildvcs=false -ldflags="-s -w" -o /out/vnet-migrate ./cmd/migrate \
 && CGO_ENABLED=0 GOOS=linux go build -buildvcs=false -ldflags="-s -w" -o /out/vnet-seed ./cmd/seed

# ------------------------------------------------------------------- runtime
FROM alpine:3.20

# tzdata is not optional: the backend resolves Asia/Ho_Chi_Minh at runtime and
# silently falls back to UTC without it, which would shift curfew windows,
# time-of-day pricing and every scheduled job.
# postgresql-client provides the pg_dump/pg_restore the backup feature shells out to.
RUN apk add --no-cache ca-certificates tzdata postgresql16-client \
    && addgroup -S vnet && adduser -S -G vnet vnet

ENV TZ=Asia/Ho_Chi_Minh

WORKDIR /app

COPY --from=server /out/vnet-server /app/vnet-server
COPY --from=server /out/vnet-migrate /app/vnet-migrate
COPY --from=server /out/vnet-seed /app/vnet-seed

# Uploaded files live here and are mounted as a volume in compose.
RUN mkdir -p /app/uploads && chown -R vnet:vnet /app

USER vnet

EXPOSE 8080

# /api/health is unauthenticated. wget comes from busybox in the base image,
# so no extra package is needed.
HEALTHCHECK --interval=15s --timeout=3s --start-period=20s --retries=3 \
  CMD wget -q -O /dev/null http://127.0.0.1:8080/api/health || exit 1

ENTRYPOINT ["/app/vnet-server"]
