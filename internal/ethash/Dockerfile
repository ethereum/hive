# Docker container spec for building an ethash DAG for the very first epoch that
# is needed by the various simulator to prevent miners from stalling till eternity.
#
# Callers need to:
#   - Bind /root/.ethash to an external volume for cache reuse
#   - Forward UID envvar to reown newly generated ethash files
FROM alpine:3.4

# Inject the DAG generator script
ADD ethash.go ethash.go

# Build the DAG maker and remove all dependencies
ENV GOPATH /gopath

RUN \
  apk add --update go git gcc musl-dev && \
	go get github.com/ethereum/ethash    && \
  go build -o ethash ethash.go         && \
  apk del go git gcc musl-dev          && \
  rm -rf /var/cache/apk/* $GOPATH/*

# Define the tiny startup script to generate the DAG and reown it
RUN \
  echo '#!/bin/sh'                          > /ethash.sh && \
	echo 'set -e'                            >> /ethash.sh && \
	echo '/ethash'                           >> /ethash.sh && \
  echo 'if [ "$UID" != "0" ]; then'        >> /ethash.sh && \
	echo '  adduser -u $UID -D ethash'       >> /ethash.sh && \
	echo '  chown -R ethash /root/.ethash/*' >> /ethash.sh && \
  echo 'fi'                                >> /ethash.sh && \
	chmod +x /ethash.sh

ENTRYPOINT ["/ethash.sh"]
