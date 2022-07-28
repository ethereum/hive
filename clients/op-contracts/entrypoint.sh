#!/bin/bash

cleanup() {
    echo "Clean up"
    rm -rf "$dest"
}
trap cleanup EXIT
trap "exit 1" INT ERR
