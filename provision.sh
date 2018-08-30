#!/usr/bin/env bash

set -e

# install required Docker dependencies
echo "Running as $(whoami)"
sudo apt-get update
sudo apt-get install -y \
    apt-transport-https \
    ca-certificates \
    curl \
    software-properties-common \
    build-essential \
    git

# install Docker GPG key and repo
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo apt-key add -
sudo apt-key fingerprint | grep 0EBFCD88
if [ $? -eq 1 ]
then
    echo "Invalid Docker GPG key."
    exit 1
fi
sudo add-apt-repository \
   "deb [arch=amd64] https://download.docker.com/linux/ubuntu \
   $(lsb_release -cs) \
   stable"

# install Docker
sudo apt-get update
sudo apt-get install -y docker-ce
# allow vagrant user to interact with Docker
sudo usermod -aG docker vagrant

# install go
curl https://dl.google.com/go/go1.11.linux-amd64.tar.gz --output go.tgz
sudo tar -C /usr/local -xaf go.tgz
sudo mkdir -p /go/bin
sudo mkdir -p /go/src/github.com/karalabe/hive
sudo chown -R vagrant:vagrant /go
cat >> /home/vagrant/.bashrc <<EOF
export GOPATH=/go
export GOBIN=/go/bin
export PATH="\$PATH:\$GOBIN:/usr/local/go/bin"
EOF
export GOPATH=/go
export GOBIN=/go/bin
export PATH="$PATH:$GOBIN:/usr/local/go/bin"
echo "GOPATH: $GOPATH"
echo "GOBIN: $GOBIN"
echo "PATH: $PATH"

# Compile Hive
echo "Compiling..."
cd /go/src/github.com/karalabe/hive
go install ./
