# Docker image with netcat for relaying traffic
FROM alpine:latest
RUN apk add --update bash 
RUN apk add socat
ADD relay.sh /relay.sh

ENTRYPOINT ["/relay.sh"]
