FROM golang:1-alpine as builder
RUN apk add --update gcc musl-dev linux-headers
ADD . /source
WORKDIR /source
RUN go build -o hive-proxy ./tool

FROM alpine:latest
COPY --from=builder /source/hive-proxy .
EXPOSE 8081/tcp
ENTRYPOINT ./hive-proxy --addr :8081
