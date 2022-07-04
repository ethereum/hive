FROM golang:1-alpine AS builder
RUN apk --no-cache add gcc musl-dev linux-headers cmake make clang build-base clang-static clang-dev
ADD . /source
WORKDIR /source
RUN go build -o ./sim .

# Build the runner container.
FROM alpine:latest
ADD . /
COPY --from=builder /source/sim /
ENTRYPOINT ["./sim"]
