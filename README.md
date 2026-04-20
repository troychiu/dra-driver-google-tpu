# TPU Resource Driver for Dynamic Resource Allocation (DRA)

This repository contains a TPU resource driver for use with the [Dynamic
Resource Allocation
(DRA)](https://kubernetes.io/docs/concepts/scheduling-eviction/dynamic-resource-allocation/)
feature of Kubernetes.

## Quickstart and Demo

The driver itself provides access to a set of mock TPU devices, which can be
changed to a specific TPU device gen that you choose, and this demo
walks through the process of building and installing the driver followed by
running a set of workloads that consume these TPUs.

### Prerequisites

* [GNU Make 3.81+](https://www.gnu.org/software/make/)
* [GNU Tar 1.34+](https://www.gnu.org/software/tar/)
* [docker v20.10+ (including buildx)](https://docs.docker.com/engine/install/) or [Podman v4.9+](https://podman.io/docs/installation)
* [helm v3.7.0+](https://helm.sh/docs/intro/install/)
* [kubectl v1.18+](https://kubernetes.io/docs/reference/kubectl/)

### Demo
We start by first cloning this repository and `cd`ing into it. All of the
scripts and example Pod specs used in this demo are contained here, so take a
moment to browse through the various files and see what's available:

**Note**: The scripts will automatically use either `docker`, or `podman` as the container tool command, whichever
can be found in the PATH. To override this behavior, set `CONTAINER_TOOL` environment variable either by calling
`export CONTAINER_TOOL=docker`, or by prepending `CONTAINER_TOOL=docker` to a script
(e.g. `CONTAINER_TOOL=docker ./path/to/script.sh`).

#### Building TPU DRA driver
In order to build the driver you will need to update the `DRIVER_IMAGE_REGISTRY`
in `/demo/scripts/common.sh`. This where the rest of the scripts get access to
commonly used information.

From here we will build the image for the TPU resource driver:
```bash
./demo/scripts/build-driver.sh
```

**Note**: The script will try to push the image to the registry by default. If you
are building for a Kind cluster, you can skip the push by setting the
`PUSH_IMAGE` environment variable to `false`:
```bash
PUSH_IMAGE=false ./demo/scripts/build-driver.sh
```

#### Cluster Setup
Choose one of the following three options to set up your Kubernetes cluster:

##### Option 1: Kind Cluster with fake TPU devices
This script will create a kind cluster with fake tpu devices.

```bash
./demo/clusters/kind/create-cluster.sh
```

##### Option 2: GKE Cluster with fake TPU devices
This script will create a gke cluster, then create a nodepool with tpu labels,
and then create fake tpu devices that can be found by the DRA driver and disable
tpu-device-plugin. In addition, it will enable CDI in containerd and restart it.
CDI is required for DRA to work.

```bash
./demo/clusters/gke/create-fake-tpu-cluster-for-dra.sh
```

**Note**: You will need to update this script if you have to change what TPU gen
devices that are faked and the labels that are added to the node pool. Currently
tpu-v4-podslice TPU topology is used.

##### Option 3: GKE Cluster with real TPU
If you have a project with TPU quota, you can use this script to create a GKE
cluster with v6e TPU (or any type for your specifc needs) and prepare the
cluster to be able use DRA

```bash
./demo/clusters/gke/create-tpu-cluster-for-dra.sh
```

#### Installing driver on the cluster
```bash
./demo/scripts/install-dra-driver.sh
```

**Note**: This install script uses helm to package up the neccesary components
of DRA driver and deploys to the cluster you are currently connected to.

Once installed, double check the driver components have come up successfully:
```console
$ kubectl get pod -n dra-driver-google-tpu
NAME                                        READY   STATUS    RESTARTS   AGE
dra-driver-google-tpu-kubeletplugin-55jdj   3/3     Running   0          1m
```

And show the initial state of available TPU devices on the worker node:
```
$ kubectl get resourceslice -o yaml
apiVersion: v1
items:
- apiVersion: resource.k8s.io/v1beta1
  kind: ResourceSlice
  metadata:
    creationTimestamp: "2025-01-21T18:49:28Z"
    generateName: gke-tpu-dra-cluster-tpu-node-pool-5649a47f-zs8v-tpu.google.com-
    generation: 1
    name: gke-tpu-dra-cluster-tpu-node-pool-5649a47f-zs8v-tpu.googlejh8t6
    ownerReferences:
    - apiVersion: v1
      controller: true
      kind: Node
      name: gke-tpu-dra-cluster-tpu-node-pool-5649a47f-zs8v
      uid: d19d3fac-e87d-4e92-8d57-72ae2cde65dc
    resourceVersion: "3283457"
    uid: 5087958f-fbb6-46e2-915f-7bbdfe2531df
  spec:
    devices:
    - basic:
        attributes:
          index:
            int: 0
          tpuGen:
            string: v4
          uuid:
            string: tpu-25541d5c-7c31-8412-d7cb-c8ebff2fa5c9
      name: accel0
    - basic:
        attributes:
          index:
            int: 1
          tpuGen:
            string: v4
          uuid:
            string: tpu-25541d5c-7c31-8412-d7cb-c8ebff2fa5c9
      name: accel1
    - basic:
        attributes:
          index:
            int: 2
          tpuGen:
            string: v4
          uuid:
            string: tpu-25541d5c-7c31-8412-d7cb-c8ebff2fa5c9
      name: accel2
    - basic:
        attributes:
          index:
            int: 3
          tpuGen:
            string: v4
          uuid:
            string: tpu-25541d5c-7c31-8412-d7cb-c8ebff2fa5c9
      name: accel3
    driver: tpu.google.com
    nodeName: gke-tpu-dra-cluster-tpu-node-pool-5649a47f-zs8v
    pool:
      generation: 1
      name: gke-tpu-dra-cluster-tpu-node-pool-5649a47f-zs8v
      resourceSliceCount: 1
kind: List
metadata:
  resourceVersion: ""
```

#### Deploy workload that request TPU though resource claims

Deploy pods that request one TPU resource to demonstrate how
`ResourceClaim`s and `ResourceClaimTemplate`s can be used to
select and configure resources in various ways:

Deploys 1 pod with 1 container which requests all tpus on node
resource
```bash
kubectl apply -f demo/specs/tpu-test.yaml
```
Deploys 1 pod with 1 container that requests all 4 TPU resources with one claim
Used to test with real tpu devices

**Note**: Before applying `vllm-tpu.yaml`, you must edit the file and replace `REPLACE_WITH_YOUR_HUGGING_FACE_TOKEN` with your actual Hugging Face token.

```bash
kubectl apply -f demo/specs/vllm-tpu.yaml
```

Verfiy that all pods are running successfully
```bash
kubectl get pods -A
```

Then verify that the TPU devices were correctly injected into each pod
as requested:
```bash
for pod in $(kubectl get pod -n tpu-test --output=jsonpath='{.items[*].metadata.name}'); do \
    for ctr in $(kubectl get pod -n tpu-test ${pod} -o jsonpath='{.spec.containers[*].name}'); do \
      echo "${pod} ${ctr}:"
      kubectl exec -n tpu-test ${pod} -c ${ctr} -- ls -l /dev/ | grep -E "accel|tpu" || echo "No TPU devices found"
    done
done
```

**Note**: You can use these pod templates a starting point on how to use create
Resource claim templates and use them to request TPU resources

## References

For more information on the DRA Kubernetes feature and developing custom resource drivers, see the following resources:

* [Dynamic Resource Allocation in Kubernetes](https://kubernetes.io/docs/concepts/scheduling-eviction/dynamic-resource-allocation/)
* [Example DRA Driver](https://github.com/kubernetes-sigs/dra-example-driver)

## Community, discussion, contribution, and support

Learn how to engage with the Kubernetes community on the [community page](http://kubernetes.io/community/).

You can reach the maintainers of this project at:

- [Slack channel](https://kubernetes.slack.com/messages/sig-node)
- [Mailing List](https://groups.google.com/a/kubernetes.io/g/sig-node)

### Code of conduct

Participation in the Kubernetes community is governed by the [Kubernetes Code of Conduct](code-of-conduct.md).
