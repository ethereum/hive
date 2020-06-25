Running it during development:
```
docker run -it offcode/private-go-ethereum:latest --rpcapi eth,admin,dev -vmodule p2p/discover=5 --netrestrict 192.168.0.122/24
```