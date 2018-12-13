#!/bin/bash

set -eu

echo "ETCD version:"
curl --insecure \
    --fail \
    --silent \
    --show-error \
    -L \
    --cacert /ssl/etcd-client-ca.crt \
    --key /ssl/etcd-client.key \
    --cert /ssl/etcd-client.crt \
    "https://${ETCD_SERVER}/version"

echo "\nStarting test"

etcd_server="${ETCD_SERVER:-host.docker.internal:2379}"
keys=${KEY_COUNT:-100}
size=${KEY_SIZE:-1000}
opt="-s -w %{remote_ip}:%{remote_port}\t%{time_total}\t%{size_upload}\\n -o /dev/null"

padsize=${#keys}

echo -e "#\t#ip:port\ttime_total\tsize_upload"
trap 'echo "Updated $i keys of $KEY_COUNT"' EXIT

for i in $(seq 1 ${keys}); do
    payload="-d value=$(dd if=/dev/urandom count=1 bs=$size 2>/dev/null | base64 -w0  | sed -e 's/+/-/g' -e 's/\//_/g' )"
    args=$(printf "%0${padsize}d" $i)
    url="https://${etcd_server}/v2/keys/stressetcd/${args}"
    printf '%-8s' "$i"
    curl --insecure \
	--fail \
	-X PUT \
	-L ${opt} ${payload}  \
	--cacert /ssl/etcd-client-ca.crt \
	--key /ssl/etcd-client.key \
	--cert /ssl/etcd-client.crt \
	${url}
done
