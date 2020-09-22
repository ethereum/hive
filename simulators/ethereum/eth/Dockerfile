ARG branch=latest
FROM ethereum/client-go:$branch

ADD testchain-data /testchain-data/

ENTRYPOINT /usr/local/bin/geth --datadir=./testchain-data/ --verbosity=5 --nat=none --nodiscover --nousb --http --http.port 7474 --http.api admin