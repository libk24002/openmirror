FROM golang:1.22-bookworm AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/openmirror ./cmd/openmirror

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /
COPY --from=builder /out/openmirror /openmirror

EXPOSE 8080
ENTRYPOINT ["/openmirror"]
