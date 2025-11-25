FROM golang:1.25-alpine AS builder

ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

WORKDIR /build

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN go build -o /app ./cmd/raft/main.go

FROM alpine:3.21 AS final

COPY --from=builder /app /bin/app

EXPOSE 5889

CMD ["bin/app"]