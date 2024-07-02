FROM golang:1.22.4 as builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w -X main.version=$VERSION" -o /ethereum-rpc-checker ./cmd/ethereum-rpc-checker

FROM alpine

COPY --from=builder /ethereum-rpc-checker /ethereum-rpc-checker

ENTRYPOINT ["/ethereum-rpc-checker"]
