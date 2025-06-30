# Build stage
FROM golang:1.24-alpine AS builder

ARG TARGET_CMD

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /configuration-server ./${TARGET_CMD}

# Final scratch image
FROM scratch

COPY --from=builder /configuration-server /configuration-server
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

EXPOSE 4000

ENTRYPOINT ["/configuration-server"]

