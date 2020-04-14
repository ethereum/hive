# This dockerfile is the root "shell" around the entire hive container ecosystem.
#
# Its goal is to containerize everything that hive does within a single mega
# container, preventing any leakage of junk (be that file system, docker images
# and/or containers, network traffic) into the host system.
#
# To this effect is runs its own docker engine within, executing the entire hive
# test suite inside. The data workspace of the internal docker engine needs to
# be mounted outside to allow proper image caching. Further, to allow running a
# docker instance internally, this shell must be started in privileged mode.
#
# Callers need to:
#   - Bind /var/lib/docker to an external volume for cache reuse
#   - Forward UID envvar to reown docker and hive generated files
#   - Run with --privileged to allow docker-in-docker containers
FROM docker:dind

# Configure the container for building hive
RUN apk add --update musl-dev go && rm -rf /var/cache/apk/*
ENV GOPATH /gopath
ENV PATH   $GOPATH/bin:$PATH
ENV GO111MODULE=on

# We need geth
# Build go-ethereum on the fly and delete all build tools afterwards
RUN \
	apk add --update  git	&& \
	go get github.com/ethereum/go-ethereum && \
        apk del git

#        git clone https://github.com/ethereum/go-ethereum $GOPATH/src/github.com/ethereum/go-ethereum && \


# Inject and build hive itself (modified during hive dev only, cache builds)
ADD . $GOPATH/src/github.com/ethereum/hive

WORKDIR $GOPATH/src/github.com/ethereum/hive
RUN go install
RUN go install ./chaintools

# Define the tiny startup script to boot docker and hive afterwards
RUN \
   echo '#!/bin/sh'  > $GOPATH/bin/hive.sh && \
	echo 'set -e'    >> $GOPATH/bin/hive.sh && \
	\
	echo 'dockerd-entrypoint.sh --storage-driver=aufs 2>/dev/null &' >> $GOPATH/bin/hive.sh && \
	echo 'while [ ! -S /var/run/docker.sock ]; do sleep 1; done'           >> $GOPATH/bin/hive.sh && \
	\
	echo 'for id in `docker ps -a -q`; do docker rm -f $id; done'                                                     >> $GOPATH/bin/hive.sh && \
	echo 'for id in `docker images -f "dangling=true" | tail -n +2 | awk "{print \\$3}"`; do docker rmi -f $id; done' >> $GOPATH/bin/hive.sh && \
	echo 'hive --docker-noshell $@'                                                                                   >> $GOPATH/bin/hive.sh && \
	echo 'for id in `docker ps -a -q`; do docker rm -f $id; done'                                                     >> $GOPATH/bin/hive.sh && \
	echo 'for id in `docker images -f "dangling=true" | tail -n +2 | awk "{print \\$3}"`; do docker rmi -f $id; done' >> $GOPATH/bin/hive.sh && \
	\
	echo 'adduser -u $UID -D hive'       >> $GOPATH/bin/hive.sh && \
	echo 'chown -R hive /var/lib/docker' >> $GOPATH/bin/hive.sh && \
  echo 'chown -R hive workspace'       >> $GOPATH/bin/hive.sh && \
	\
	chmod +x $GOPATH/bin/hive.sh

ENTRYPOINT ["hive.sh"]

# Inject all other runtime resources (modified most frequently)
COPY . $GOPATH/src/github.com/ethereum/hive
