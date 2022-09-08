ARG branch=develop
FROM us-central1-docker.pkg.dev/bedrock-goerli-development/images/op-batcher:$branch

RUN apk add bash

COPY ./entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

RUN op-batcher --version 2>/dev/null | head -1 > /version.txt

ENTRYPOINT ["/bin/bash", "/entrypoint.sh"]
