# Using Prometheus with your MongoDB Resource

We have added a sample yaml file that you could use to deploy a MongoDB resource
in your Kubernetes cluster, with a
[`ServiceMonitor`](https://github.com/prometheus-operator/prometheus-operator/blob/main/Documentation/user-guides/getting-started.md#related-resources)
to indicate Prometheus how to consume metrics data from it.

This is a simple MongoDB resource with one user, and with the `spec.Prometheus`
attribute with basic HTTP Auth and no TLS, that will allow you to test
Prometheus metrics coming from MongoDB.

## Quick Start

We have tested this setup with versuion 0.54 of the [Prometheus
Operator](https://github.com/prometheus-operator/prometheus-operator).

### Installing Prometheus Operator

The Prometheus Operator can be installed using Helm. Find the installation
instructions
[here](https://github.com/prometheus-community/helm-charts/tree/main/charts/kube-prometheus-stack#kube-prometheus-stack):

What I did was:

``` shell
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update

helm install prometheus prometheus-community/kube-prometheus-stack --namespace prometheus-system --create-namespace
```

### Installing MongoDB

*Change after release to a proper Helm install.*

* Create a Namespace to hold our MongoDB Operator and Resources

``` shell
kubectl create namespace mongodb
```

* Follow the [Installation Instructions](https://github.com/mongodb/mongodb-kubernetes-operator/blob/master/docs/install-upgrade.md#operator-in-same-namespace-as-resources)

## Creating a MongoDB Resource

We have created a sample yaml definition that you can use to create a MongoDB
resource and a `ServiceMonitor` that will indicate Prometheus to start scraping
its metrics information.

You can apply it directly with:

``` shell
kubectl apply -f mongodb-prometheus-sample.yaml
```

This will create 2 `Secrets` containing authentication for a new MongoDB user
and Basic HTTP Auth for the Prometheus endpoint. All of this in the `mongodb`
Namespace.

It will also create a `ServiceMonitor` that will configure Prometheus to consume
this resurce's metrics. This will be created in the `prometheus-system`
namespace.
