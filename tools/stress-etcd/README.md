## About

stress-etcd populates keys in etcd database. It can be used as a simple tool
to evaluate performance of an etcd cluster

>**NOTE:** In version 0.5.0 of Kyma there was a change in naming convention of the etcd cluster.
Pods name changed from "core-catalog-etcd-stateful-X" to "service-catalog-etcd-stateful-X". If you need
to test the older version please modify content of get-ssl-files.sh script and the port-forward command.

## Usage

### Quick version
To run the test you have to perform following steps:
1. Establish a connection to the etcd cluster
2. Download certificates using `get-ssl-files.sh`
3. Run the `run-test.sh` script

### Full version

To test performance of an etcd cluster you first need to establish a connection.
To connect to etcd instance running on K8S cluster you have to forward a port from 
one of etcd instances. 

This command opens port `2379` on localhost: 

```bash
kubectl -n kyma-system port-forward service-catalog-etcd-stateful-0 2379:2379
```

To download certificates from etcd instance execute `get-ssl-files.sh` script.
It creates a subdirectory `ssl` that contains 3 files:
etcd-client.crt, etcd-client.key, etcd-client-ca.crt. 

Executing the `run-test.sh` triggers the test with default settings. 
It spawns the container, mounts the ssl directory and supplies following parameters:
* ETCD_SERVER - the address of the server (default: host.docker.internal:2379)
* KEY_COUNT - number of keys to populate (default: 100)
* KEY_SIZE - size of each key in bytes (default: 1000)

## Examples

During execution of the test you can see the following information:

```bash
$ ./run-test.sh
ETCD version:
{"etcdserver":"3.3.9","etcdcluster":"3.3.0"}
Starting test
#	#ip:port        	time_total	size_upload
1       192.168.65.2:2379	0.300764	2676
2       192.168.65.2:2379	0.246349	2676
3       192.168.65.2:2379	0.774941	2676
4       192.168.65.2:2379	0.370848	2676
5       192.168.65.2:2379	0.293486	2676
Updated 5 keys of 5

real	0m5.381s
user	0m0.032s
sys	0m0.017s
``` 

In case you want to test multiple workers in the same time you
have to spin up several instances of the `run-test.sh` script.

During execution you can observe load on the etcd cluster:
![](docs/assets/example-test-results.png)
The graphs show load on a cluster in the following configuration:
- 3 etcd nodes, ram=512MB, snapshot-count=10000
- 2 stress-etcd workers, key_size=2000

## Building

Execute script `build.sh`, it creates a new docker image named "stress-etcd"

