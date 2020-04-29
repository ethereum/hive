from nethermind/nethermind:alpine
#FROM ubuntu:18.04 AS clone
#ARG branch=master
#WORKDIR /src

#RUN echo "Cloning branch $branch from https://github.com/NethermindEth/nethermind"
#RUN apt update -y && apt install -y  curl git jq  && \ 
#    git clone  --branch $branch https://github.com/NethermindEth/nethermind && \
#    cd nethermind && git submodule update --init && \
#    (echo "{}"                                                      \
#    | jq ".+ {\"repo\":\"$(git config --get remote.origin.url)\"}" \
#    | jq ".+ {\"branch\":\"$(git rev-parse --abbrev-ref HEAD)\"}"  \
#    | jq ".+ {\"commit\":\"$(git rev-parse HEAD)\"}"               \
#	    > /version.json)
#FROM microsoft/dotnet:2.2-sdk AS build
#COPY --from=clone /src .
#COPY --from=clone /version.json .
#
#RUN cd nethermind/src/Nethermind/Nethermind.Runner && \
#    dotnet publish -c release -o out
#FROM microsoft/dotnet:2.2-aspnetcore-runtime
#RUN apt update -y && apt install -y libsnappy-dev libc6-dev libc6 unzip jq wget
#
#COPY --from=build /nethermind/src/Nethermind/Nethermind.Runner/out .
#COPY --from=build /version.json .

RUN apk add jq bash

#RUN wget https://github.com/stedolan/jq/releases/download/jq-1.6/jq-linux64 -O /usr/local/bin/jq && \
#    echo 'af986793a515d500ab2d35f8d2aecd656e764504b789b66d7e1a0b727a124c44  /usr/local/bin/jq' | sha256sum -c && \
#    chmod +x /usr/local/bin/jq

ADD enode.sh /enode.sh
ADD nethermind.sh /nethermind.sh
ADD mapper.jq /mapper.jq
ADD test.json /chainspec/test.json
ADD test.cfg /configs/test.cfg
RUN chmod +x /nethermind.sh
RUN chmod +x /enode.sh

# ENV NETHERMIND_CONFIG mainnet
# ENV NETHERMIND_DETACHED_MODE true
ENV NETHERMIND_ENODE_IPADDRESS 0.0.0.0
ENV NETHERMIND_HIVE_ENABLED true
# ENV NETHERMIND_HIVECONFIG_GENESISFILEPATH=genesis.json
ENV NETHERMIND_INITCONFIG_JSONRPCENABLED true
ENV NETHERMIND_INITCONFIG_P2PPORT 30303
ENV NETHERMIND_INITCONFIG_DISCOVERYPORT 30303
#ENV NETHERMIND_URL http://*:8545


EXPOSE 8545 30303 30303/udp

ENTRYPOINT ["/nethermind.sh"]
