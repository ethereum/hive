# Build the simulator.
FROM golang:1-alpine AS builder
RUN apk --no-cache add gcc musl-dev linux-headers
ADD . /ronin
WORKDIR /ronin
RUN go build .
ENTRYPOINT ["./ronin"]
#
## Build the runner container.
#FROM alpine:latest
#ADD . /
#COPY --from=builder /ronin/ronin /
#RUN mkdir ronin
#ENTRYPOINT ["./ronin"]
