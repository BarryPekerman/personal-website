# Build stage
FROM golang:1.26-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY content ./content
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/portfolio ./cmd/server

# Runtime stage: no shell, no package manager, runs as non-root
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/portfolio ./portfolio
COPY web ./web

EXPOSE 8080
ENTRYPOINT ["/app/portfolio"]
