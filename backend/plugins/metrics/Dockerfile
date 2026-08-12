FROM golang:1.26-alpine AS builder

WORKDIR /build

# Copy go module files first for layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build the standalone binary
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -o /metrics-api \
    ./plugins/metrics/cmd/

# ---- runtime image ----
FROM gcr.io/distroless/static:nonroot

COPY --from=builder /metrics-api /metrics-api

EXPOSE 8181
ENTRYPOINT ["/metrics-api"]
