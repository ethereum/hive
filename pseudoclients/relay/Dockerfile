# Docker image with netcat for relaying traffic
FROM alpine:latest
RUN apk add --update bash 
RUN apk add socat
ADD ./relay/relay.sh /relay.sh
RUN chmod +x /relay.sh

ENTRYPOINT ["/relay.sh"]
