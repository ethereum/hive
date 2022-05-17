FROM golang:1-alpine

ADD stdio-proxy.go .
RUN go build -o hive-stdio-proxy stdio-proxy.go

EXPOSE 8081/tcp
ENTRYPOINT ["./hive-stdio-proxy", "--addr", ":8081"]
