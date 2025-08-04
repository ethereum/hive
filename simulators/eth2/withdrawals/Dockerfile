# Build the simulator binary
FROM golang:1.22-alpine AS builder
ARG GOPROXY
ENV GOPROXY=${GOPROXY}

RUN apk --no-cache add gcc musl-dev linux-headers cmake make clang build-base clang-static clang-dev

# Prepare workspace.
# Note: the build context of this simulator image is the parent directory!
ADD . /source

# Build within simulator folder
WORKDIR /source/withdrawals
RUN go build -o ./sim .

# Build the runner container.
FROM alpine:latest
ADD . /
COPY --from=builder /source/withdrawals/sim /

ENTRYPOINT ["./sim"]
