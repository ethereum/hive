FROM golang:1-alpine as builder
ADD . /source
WORKDIR /source
RUN go build ./tool

FROM alpine:latest
COPY --from=builder /source/tool .
EXPOSE 8081/tcp
ENTRYPOINT ["./tool" "--addr", ":8081"]
