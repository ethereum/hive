# Build the simulator binary
FROM golang:1-alpine AS builder
RUN apk --no-cache add gcc musl-dev linux-headers cmake make clang build-base clang-static clang-dev

# Prepare workspace
ADD ./withdrawals/hive.go.work /source/go.work
ADD ./common /source/common
ADD ./withdrawals /source/withdrawals

# Build within simulator folder
WORKDIR /source/withdrawals
RUN go build -o ./sim .

# Build the runner container.
FROM alpine:latest
ADD . /
COPY --from=builder /source/withdrawals/sim /

ENTRYPOINT ["./sim"]
