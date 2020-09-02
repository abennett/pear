FROM golang:1.15-alpine as builder

WORKDIR /build

COPY go.mod go.sum ./

RUN go mod download

COPY *.go ./

RUN CGO_ENABLED=0 go build

FROM scratch

COPY --from=builder /build/pear .
COPY ./sql ./sql

ENTRYPOINT ["./pear"]
