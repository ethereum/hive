ARG branch=develop
FROM us-central1-docker.pkg.dev/bedrock-goerli-development/images/op-proposer:$branch

RUN apk add curl jq bash
RUN apk add --no-cache ca-certificates

COPY ./entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

RUN op-proposer --version 2>/dev/null | head -1 > /version.txt

ENTRYPOINT ["/bin/bash", "/entrypoint.sh"]
