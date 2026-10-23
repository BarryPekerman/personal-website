# Build stage
# Build on the runner's own architecture and cross-compile for the target
# (the cluster nodes are arm64), so no emulation is needed
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build
ARG TARGETOS TARGETARCH
# The git commit shown on the status panel
ARG COMMIT=dev
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY content ./content
COPY internal ./internal
COPY web ./web
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w -X main.commit=${COMMIT}" -o /out/portfolio ./cmd/server

# Runtime stage: no shell, no package manager, runs as non-root
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/portfolio ./portfolio

EXPOSE 8080
ENTRYPOINT ["/app/portfolio"]
