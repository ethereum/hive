#!/bin/bash

set -e

# The annotation is passed as JSON encoded payload in the first shell argument,
# and then passed on as body to the API POST request.
# See https://grafana.com/docs/grafana/latest/developers/http_api/annotations/
wget --post-data="$1" --header "Content-Type: application/json" "http://127.0.0.1:3000/api/annotations"
