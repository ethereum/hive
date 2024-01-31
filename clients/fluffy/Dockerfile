ARG branch=amd64-master-latest

FROM statusim/nimbus-fluffy:$branch

ADD fluffy.sh /fluffy.sh
RUN chmod +x /fluffy.sh

RUN echo "latest" > /version.txt

EXPOSE 8545 9009/udp

ENTRYPOINT ["/fluffy.sh"]
