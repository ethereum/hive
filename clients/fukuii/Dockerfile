# Hive client adapter for Fukuii (https://github.com/chippr-robotics/fukuii), built from the
# published image. Build from source instead with Dockerfile.git.
#
# `tag` picks the image (default latest, the head of fukuii's main); set it from a
# --client-file entry's build_args, e.g. tag: sha-<commit> or a release version.
#
# This directory is the single source of ethereum/hive's clients/fukuii: keep the two identical
# (scripts/hive/check-upstream-adapter.sh, workflow hive-upstream-definition.yml).

ARG baseimage=chipprbots/fukuii
ARG tag=latest
FROM $baseimage:$tag

USER root

# JDK-8385993: C2 ReduceAllocationMerges can silently miscompile field reads on
# JDK 23+ (fixed in JDK 28). JDK_JAVA_OPTIONS is read by every `java` launch in
# this container, including the AOT training run. Set here as well as in the base
# image so an older base image cannot drop it. fukuii.sh passes the
# same flags explicitly. Side effect: java prints
# "NOTE: Picked up JDK_JAVA_OPTIONS: ..." on stderr at startup.
ENV JDK_JAVA_OPTIONS="-XX:+UnlockDiagnosticVMOptions -XX:-ReduceAllocationMerges"

RUN apt-get update && apt-get install -y --no-install-recommends \
    jq \
    && rm -rf /var/lib/apt/lists/*

COPY fukuii.sh /fukuii.sh
COPY mapper.jq /mapper.jq
COPY dao-drain-list.txt /dao-drain-list.txt
COPY enode.sh /hive-bin/enode.sh
RUN chmod +x /fukuii.sh /hive-bin/enode.sh

# JDK 25 AOT cache (JEP 483/514). hive starts a fresh container for every test, and
# loading/linking ~12k classes from the assembly is a large share of each startup, so
# train once here and let every container reuse it (readiness 5.35 s -> 3.8 s median).
# aot-train.sh boots the node through /fukuii.sh itself (same java, same flags, same
# -jar path — the JVM ignores a cache whose inputs do not match), stops it once :8545
# is serving, and deletes everything the run wrote except /app/fukuii/fukuii.aot.
# fukuii.sh passes -XX:AOTCache only when that file exists.
# This step must stay AFTER the fukuii.sh COPY. It is keyed on the FROM layer, so a
# new base image (new jar) always retrains; a cache from another jar is never reused.
COPY aot-train.sh aot-train-genesis.json /
RUN chmod +x /aot-train.sh && /aot-train.sh && rm /aot-train.sh

# Unified state layout guard (spec 018 Track C, B-314): off by default. A hive --client-file
# entry with `build_args: {FUKUII_PATH_LIVE: "1"}` turns it on; fukuii.sh then passes
# -Dfukuii.state.path-live=true. Declared AFTER the AOT training step so the default image,
# and its trained cache, are unchanged (and -D properties are not part of the cache's flag
# check either way). See scripts/hive/path-live-clients.sh and docs/testing/hive-local.md.
ARG FUKUII_PATH_LIVE=
ENV FUKUII_PATH_LIVE=${FUKUII_PATH_LIVE}

# /version.txt is created by the base image (chipprbots/fukuii) from
# the actual JAR filename, so a version bump propagates automatically.

EXPOSE 8545 8546 8551 30303 30303/udp

# Check JSON-RPC HTTP (8545) — the last port to bind during startup, and the one
# hive itself talks to.
#
# StdNode.startNode binds 8551 first and 8545 second, and now AWAITS both
# bindings, so 8545 accepting a connection implies 8551 already does. Gating
# readiness on 8545 therefore covers both ports.
#
# NOT what hive probes. libhive takes the readiness port from the simulator's
# per-client parameters (internal/libhive/api.go), defaulting to 8545; this
# image ENV is never consulted for that. The `ethereum/sync` sink node is probed
# on 8551 (sync.go:97), everything else on 8545. fukuii.sh passes the value hive
# actually set through as fukuii.network.readiness-port, and StdNode binds that
# port last so a successful probe implies both RPC ports are listening. This
# default only supplies the fallback when a simulator sets nothing.
ENV HIVE_CHECK_LIVE_PORT=8545

ENTRYPOINT ["/fukuii.sh"]
