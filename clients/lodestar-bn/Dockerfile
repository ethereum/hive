ARG branch=next

FROM chainsafe/lodestar:$branch

ADD lodestar_bn.sh /lodestar_bn.sh
RUN chmod +x /lodestar_bn.sh

RUN node /usr/app/node_modules/.bin/lodestar --version | grep "Version:.*" | grep -o "v.*" > /version.txt

ENTRYPOINT ["/lodestar_bn.sh"]
