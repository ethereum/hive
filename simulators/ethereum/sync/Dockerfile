FROM golang:1-alpine

# Add the test code.
ADD /ethereum/sync/ /sync
ADD /ethereum/sync/simplechain /source/simplechain

# Add the entry script
WORKDIR /sync
RUN go build -v ./sync

ENTRYPOINT ["./sync"]
