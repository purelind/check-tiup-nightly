FROM golang:1.23.3-bookworm AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN make build

FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /app/build/server .

EXPOSE 8080

CMD ["./server"]
