ARG branch=next

FROM chainsafe/lodestar:$branch

ADD lodestar_vc.sh /lodestar_vc.sh
RUN chmod +x /lodestar_vc.sh

RUN node /usr/app/node_modules/.bin/lodestar --version | grep "Version:.*" | grep -o "v.*" > /version.txt

ENTRYPOINT ["/lodestar_vc.sh"]
