FROM gcr.io/evmchain/internal-devnet-l1-node:latest

ENV CHAIN_ID=31336
ENV PERIOD=12
ENV NODE_KEY="9c52ecfdf0397641379694b367a2bf4328580e4310753d4064ab0af8b132d565"

RUN /usr/local/bin/geth console --exec 'console.log(admin.nodeInfo.name)' --maxpeers=0 --nodiscover --dev 2>/dev/null | head -1 >/version.txt
