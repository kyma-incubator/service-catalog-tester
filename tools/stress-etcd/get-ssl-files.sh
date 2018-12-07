#!/bin/bash

set -eu

rm -rf ./ssl
kubectl cp kyma-system/service-catalog-etcd-stateful-0:etc/etcdtls/operator/etcd-tls ./tmp
mv tmp/..20*/ ./ssl/
rm -rf ./tmp
