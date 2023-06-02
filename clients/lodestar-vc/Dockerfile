ARG tag=next
ARG baseimage=chainsafe/lodestar

FROM $baseimage:$tag

ADD lodestar_vc.sh /lodestar_vc.sh
RUN chmod +x /lodestar_vc.sh

RUN node /usr/app/node_modules/.bin/lodestar --version | grep "Version:.*" | grep -o "v.*" > /version.txt

ENTRYPOINT ["/lodestar_vc.sh"]
