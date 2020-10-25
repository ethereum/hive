FROM golang:1-alpine
RUN apk add --update git make gcc musl-dev curl linux-headers

# Add the test code.
ADD /ethereum/sync/ /sync
ADD /ethereum/sync/simplechain /source/simplechain

# Add the entry script
WORKDIR /sync
RUN go build -v

ENTRYPOINT ["./sync"]
