#!/bin/bash

# Immediately abort the script on any error encountered
set -e

lighthouse --version | head -1 | sed "s/ /\//g"
