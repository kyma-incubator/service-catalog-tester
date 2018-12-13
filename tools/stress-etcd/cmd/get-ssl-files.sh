#!/bin/bash

set -eu

readonly ROOT_PATH=$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )

TMP_DIR=${ROOT_PATH}/tmp
SSL_DIR=${ROOT_PATH}/ssl

rm -rf ${SSL_DIR} ${TMP_DIR}
kubectl cp kyma-system/service-catalog-etcd-stateful-0:etc/etcdtls/operator/etcd-tls ${TMP_DIR}
mv ${TMP_DIR}/..20*/ ${SSL_DIR}
rm -rf ${TMP_DIR}
