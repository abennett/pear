FROM golang:1.17-alpine as builder

RUN apk update \
    && apk add --no-cache ca-certificates \
    && update-ca-certificates 2>/dev/null

WORKDIR /build

COPY go.mod go.sum ./

RUN go mod download

COPY *.go ./

RUN CGO_ENABLED=0 go build -trimpath -ldflags="-w -s"

FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /build/pear .
COPY ./sql ./sql

ENTRYPOINT ["./pear"]
