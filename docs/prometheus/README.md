# Using Prometheus with your MongoDB Resource

We have added a sample yaml file that you could use to deploy a MongoDB resource
in your Kubernetes cluster, with a
[`ServiceMonitor`](https://github.com/prometheus-operator/prometheus-operator/blob/main/Documentation/user-guides/getting-started.md#related-resources)
to indicate Prometheus how to consume metrics data from it.

This is a simple MongoDB resource with one user, and with the `spec.Prometheus`
attribute with basic HTTP Auth and no TLS, that will allow you to test
Prometheus metrics coming from MongoDB.

## Quick Start

We have tested this setup with version 0.54 of the [Prometheus
Operator](https://github.com/prometheus-operator/prometheus-operator).

### Installing Prometheus Operator

The Prometheus Operator can be installed using Helm. Find the installation
instructions
[here](https://github.com/prometheus-community/helm-charts/tree/main/charts/kube-prometheus-stack#kube-prometheus-stack):

This can be done with:

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


## Bonus: Enable TLS on the Prometheus Endpoint

### Installing Cert-Manager

We will install [Cert-Manager](https://cert-manager.io/) from using Helm.

``` shell
helm repo add jetstack https://charts.jetstack.io
helm repo update
helm install \
  cert-manager jetstack/cert-manager \
  --namespace cert-manager \
  --create-namespace \
  --version v1.7.1 \
  --set installCRDs=true
```

Now with Cert-Manager installed we we'll create a Cert-Manager `Issuer` and then
a `Certificate`. We provide 2 files that can be used to create a new `Issuer`.

First we need to create a `Secret` holding a TLS certificate `tls.crt` and
`tls.key` entries. We provide the certificate and key files that can be used to
create a Cert-Manager `Certificate`, they are in the `testdata/tls` directory.

``` shell
$ kubectl create secret tls issuer-secret --cert=../../testdata/tls/ca.crt --key=../../testdata/tls/ca.key \
    --namespace mongodb
secret/issuer-secret created
```

And now we are ready to create a new `Issuer` and `Certificate`, by running the
following command:

``` shell
$ kubectl apply -f issuer-and-cert.yaml --namespace mongodb
issuer.cert-manager.io/ca-issuer created
certificate.cert-manager.io/prometheus-target-cert created
```

### Enabling TLS on the MongoDB CRD

<center>_Make sure this configuration is not used in Production environments! A Security
expert should be advising you on how to configure TLS_</center>

We need to add a new entry to `spec.prometheus` section of the MongoDB
`CustomResource`; we can do it executing the following
[patch](https://kubernetes.io/docs/tasks/manage-kubernetes-objects/update-api-object-kubectl-patch/)
operation.

``` shell
$ kubectl patch mdbc mongodb --type='json' \
    -p='[{"op": "add", "path": "/spec/prometheus/tlsSecretKeyRef", "value":{"name": "prometheus-target-cert"}}]' \
    --namespace mongodb

mongodbcommunity.mongodbcommunity.mongodb.com/mongodb patched
```

After a few minutes, the MongoDB resource should be back in Running phase. Now
we need to configure our Prometheus `ServiceMonitor` to point at the HTTPS
endpoint.

### Update ServiceMonitor

To update our `ServiceMonitor` we will again patch the resource:

``` shell
$ kubectl patch servicemonitors mongodb-sm --type='json' \
    -p='
[
    {"op": "replace", "path": "/spec/endpoints/0/scheme", "value": "https"},
    {"op": "add",     "path": "/spec/endpoints/0/tlsConfig", "value": {"insecureSkipVerify": true}}
]
' \
    --namespace mongodb

servicemonitor.monitoring.coreos.com/mongodb-sm patched
```

With these changes, the new `ServiceMonitor` will be pointing at the HTTPS
endpoint (defined in `/spec/endpoints/0/scheme`). We are also setting
`spec/endpoints/0/tlsConfig/insecureSkipVerify` to `true`, which will make
Prometheus to not verify TLS certificates on MongoDB's end.

Prometheus should now be able to scrape the MongoDB's target using HTTPS this
time.
