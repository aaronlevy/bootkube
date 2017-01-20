# Hack / Dev multi-node build

**Note: All scripts are assumed to be ran from this directory.**

## Quickstart

This will generate the default assets in the `cluster` directory and launch multi-node self-hosted cluster.

```
./bootkube-up
```

## Cleaning up

To stop the running cluster and remove generated assets, run:

```
vagrant destroy -f
rm -rf cluster
```

## Experimental Self Hosted Etcd

First clean up any existing assets:

```
vagrant destroy -f
rm -rf cluster
```

Then start bootkube using the `SELF_HOST_ETCD=true`:

```
SELF_HOST_ETCD=true ./bootkube-up
```

Once the cluster has launched, you can use the [etcd-operator](https://github.com/coreos/etcd-operator#etcd-operator) to manage your etcd cluster.

### Resize an etcd cluster

`kubectl apply` doesn't work for TPR at the moment. See [kubernetes/#29542](https://github.com/kubernetes/kubernetes/issues/29542).
As a workaround, we use cURL to resize the cluster.

First, use kubectl to create a reverse proxy:

```
$ kubectl --kubeconfig=cluster/auth/kubeconfig proxy --port=8080
Starting to serve on 127.0.0.1:8080
```
Now we can talk to apiserver via "http://127.0.0.1:8080".

In another terminal, create a json file with the new configuration:

```
$ cat body.json
{
  "apiVersion": "coreos.com/v1",
  "kind": "EtcdCluster",
  "metadata": {
    "name": "kube-etcd",
    "namespace": "kube-system"
  },
  "spec": {
    "size": 3
  }
}
```

Use the following command to change the cluster size from 1 to 3:

```
$ curl -H 'Content-Type: application/json' -X PUT --data @body.json http://127.0.0.1:8080/apis/coreos.com/v1/namespaces/kube-system/etcdclusters/kube-etcd
```

We should see

```
$ kubectl get pods
NAME                            READY     STATUS    RESTARTS   AGE
kube-etcd-0000       1/1       Running   0          1m
kube-etcd-0001       1/1       Running   0          1m
kube-etcd-0002       1/1       Running   0          1m
```

Now we can decrease the size of cluster from 3 back to 1.

Create a json file with cluster size of 1:

```
$ cat body.json
{
  "apiVersion": "coreos.com/v1",
  "kind": "EtcdCluster",
  "metadata": {
    "name": "kube-etcd",
    "namespace": "kube-system"
  },
  "spec": {
    "size": 1
  }
}
```

Apply it to API Server:

```
$ curl -H 'Content-Type: application/json' -X PUT --data @body.json http://127.0.0.1:8080/apis/coreos.com/v1/namespaces/kube-system/etcdclusters/kube-etcd
```

We should see that etcd cluster will eventually reduce to 1 pods:

```
$ kubectl get pods
NAME                            READY     STATUS    RESTARTS   AGE
example-etcd-cluster-0001       1/1       Running   0          1m
```

**NOTE:** If you lose access to a quorum of etcd nodes, the system will not currently recover. In the case of a single-node etcd cluster, this means the loss of that one node (even during reboot) means that it cannot recover. However, these are known tasks:

- https://github.com/kubernetes-incubator/bootkube/issues/284
- https://github.com/kubernetes-incubator/bootkube/issues/288

