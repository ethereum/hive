ARG tag=next
ARG baseimage=chainsafe/lodestar

FROM $baseimage:$tag

ADD lodestar_bn.sh /lodestar_bn.sh
RUN chmod +x /lodestar_bn.sh

RUN node /usr/app/node_modules/.bin/lodestar --version | grep "Version:.*" | grep -o "v.*" > /version.txt

ENTRYPOINT ["/lodestar_bn.sh"]
