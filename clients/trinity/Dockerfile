FROM python:3.7
ENV branch=master

# Install dependencies.
RUN apt-get -y update && apt-get install -y bash curl git make cmake file libsnappy-dev libgmp3-dev

RUN wget https://github.com/stedolan/jq/releases/download/jq-1.6/jq-linux64 -O /usr/local/bin/jq && \
    echo 'af986793a515d500ab2d35f8d2aecd656e764504b789b66d7e1a0b727a124c44  /usr/local/bin/jq' | sha256sum -c && \
    chmod +x /usr/local/bin/jq

# Set up code directory.
RUN mkdir -p /usr/src/app
WORKDIR /usr/src/app
RUN git clone --branch $branch https://github.com/ethereum/trinity .
RUN pip install -e .[dev]  --no-cache-dir
RUN pip install -U trinity --no-cache-dir

ADD genesis.json /genesis.json
ADD mapper.jq /mapper.jq
ADD trinity.sh /trinity.sh
RUN chmod +x /trinity.sh
ADD enode.sh /enode.sh
RUN chmod +x /enode.sh

# Write the version file.
RUN echo "branch: "$(git rev-parse --abbrev-ref HEAD)", commit: "$(git rev-parse HEAD)"" > /version.txt

EXPOSE  8545 8546 30303 30303/udp
CMD ["/trinity.sh"]
