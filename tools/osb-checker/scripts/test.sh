#!/bin/bash
# Usage: ./test-api-compliance.sh host port timeout

if [ -z "$1" ] || [ -z "$2" ] || [ -z "$3" ]; then echo "Usage: ./test-api-compliance.sh <host> <port> <timeout in seconds>"; exit 1; fi

set -e

HOST=$1
PORT=$2
TIMEOUT=$3

TESTS_PATH=/opt/osb-checker/2.13/tests
CONFIG_PATH=/$TESTS_PATH/test/configs/config_mock.json

# Wait for the apiserver to start responding
/app/wupiao.sh $HOST $PORT $TIMEOUT
cp /app/config_mock.json $TESTS_PATH/test/configs
cd $TESTS_PATH

exec bash -c mocha
