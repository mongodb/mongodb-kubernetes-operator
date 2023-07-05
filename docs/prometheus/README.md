# Use Prometheus with your MongoDB Resource

 You can use the [mongodb-prometheus-sample.yaml](mongodb-prometheus-sample.yaml) file to 
 deploy a MongoDB resource in your Kubernetes cluster, with a
[`ServiceMonitor`](https://github.com/prometheus-operator/prometheus-operator/blob/main/Documentation/user-guides/getting-started.md#related-resources)
to indicate to Prometheus how to consume metrics data from 
it.

The sample specifies a simple MongoDB resource with one user,
and the `spec.Prometheus` attribute with basic HTTP 
authentication and no TLS. The sample lets you test
the metrics that MongoDB sends to Prometheus.

## Quick Start

We tested this setup with version 0.54 of the [Prometheus
Operator](https://github.com/prometheus-operator/prometheus-operator).

### Prerequisites

* Kubernetes 1.16+
* Helm 3+

### Install the Prometheus Operator

You can install the Prometheus Operator using Helm. To learn 
more, see the [installation instructions](https://github.com/prometheus-community/helm-charts/tree/main/charts/kube-prometheus-stack#kube-prometheus-stack).

To install the Prometheus Operator using Helm, run the 
following commands:

``` shell
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update
helm install prometheus prometheus-community/ \   
  kube-prometheus-stack --namespace <prometheus-system> \   
  --create-namespace
```

### Install the MongoDB Community Kubernetes Operator

Run the following command to install the Community Kubernetes 
Operator and create a namespace to contain the Community 
Kubernetes Operator and resources:

``` shell
helm install community-operator mongodb/community-operator --namespace <mongodb> --create-namespace
```

To learn more, see the [Installation Instructions](../install-upgrade.md#operator-in-same-namespace-as-resources).

## Create a MongoDB Resource

 You can use the [mongodb-prometheus-sample.yaml](mongodb-prometheus-sample.yaml) file to 
 deploy a MongoDB resource in your Kubernetes cluster, with a
[`ServiceMonitor`](https://github.com/prometheus-operator/prometheus-operator/blob/main/Documentation/user-guides/getting-started.md#related-resources)
to indicate to Prometheus how to consume metrics data from 
it.

You can apply the sample directly with the following command:

``` shell
kubectl apply -f <mongodb-prometheus-sample.yaml>
```

**Note:** If you haven't cloned the 
[mongodb-kubernetes-operator](https://github.com/mongodb/mongodb-kubernetes-operator) 
repository, you must provide the full URL that points to the 
[mongodb-prometheus-sample.yaml](mongodb-prometheus-sample.yaml) file in the command:
[https://raw.githubusercontent.com/mongodb/mongodb-kubernetes-operator/master/docs/prometheus/mongodb-prometheus-sample.yaml](mongodb-prometheus-sample.yaml)

This command creates two `Secrets` that contain authentication 
for a new MongoDB user and basic HTTP authentication for the 
Prometheus endpoint. The command creates both `Secrets` in the 
`mongodb` namespace.

This command also creates a `ServiceMonitor` that configures 
Prometheus to consume this resource's metrics. This command 
creates the `ServiceMonitor` in the `prometheus-system`
namespace.

## Optional: Enable TLS on the Prometheus Endpoint

### Install Cert-Manager

1. Run the following commands to install
   [Cert-Manager](https://cert-manager.io/) using Helm:

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

2. Now with Cert-Manager installed, create a Cert-Manager 
   `Issuer` and then a `Certificate`. You can use the two files 
   that we provide to create a new `Issuer`:

   a. Run the following command to create a `Secret` that 
      contains the TLS certificate `tls.crt` and `tls.key` 
      entries. You can use the certificate and key files that 
      we provide in the [`testdata/tls`](../../testdata/tls) directory to create a Cert-Manager `Certificate`.

      ``` shell
      kubectl create secret tls issuer-secret --cert=../../testdata/tls/ca.crt --key=../../testdata/tls/ca.key \
        --namespace mongodb
      ```

      The following response appears:

      ``` shell
      secret/issuer-secret created
      ```

   b. Run the following command to create a new `Issuer` and 
      `Certificate`:

      ``` shell
      kubectl apply -f issuer-and-cert.yaml --namespace mongodb
      ```
      The following response appears:

      ``` shell
      issuer.cert-manager.io/ca-issuer created
      certificate.cert-manager.io/prometheus-target-cert created
      ```

### Enable TLS on the MongoDB CRD

**Important!** Do **NOT** use this configuration in Production 
environments! A security expert should advise you about how to 
configure TLS.

To enable TLS, you must add a new entry to the
`spec.prometheus` section of the MongoDB `CustomResource`. Run 
the following [patch](https://kubernetes.io/docs/tasks/manage-kubernetes-objects/update-api-object-kubectl-patch/)
operation to add the needed entry.

``` shell
kubectl patch mdbc mongodb --type='json' \
  -p='[{"op": "add", "path": "/spec/prometheus/tlsSecretKeyRef", "value":{"name": "prometheus-target-cert"}}]' \
  --namespace mongodb
```

The following response appears:

``` shell
mongodbcommunity.mongodbcommunity.mongodb.com/mongodb patched
```

After a few minutes, the MongoDB resource should return to the 
Running phase. Now you must configure the Prometheus 
`ServiceMonitor` to point to the HTTPS endpoint.

### Update ServiceMonitor

To update the `ServiceMonitor`, run the following command to 
patch the resource again:

``` shell
kubectl patch servicemonitors mongodb-sm --type='json' \
    -p='
[
    {"op": "replace", "path": "/spec/endpoints/0/scheme", "value": "https"},
    {"op": "add",     "path": "/spec/endpoints/0/tlsConfig", "value": {"insecureSkipVerify": true}}
]
' \
    --namespace mongodb
```

The following reponse appears:

``` shell
servicemonitor.monitoring.coreos.com/mongodb-sm patched
```

With these changes, the new `ServiceMonitor` points to the HTTPS
endpoint (defined in `/spec/endpoints/0/scheme`). You also 
set `spec/endpoints/0/tlsConfig/insecureSkipVerify` to `true`, 
so that Prometheus doesn't verify the TLS certificates on 
MongoDB's end.

Prometheus should now be able to scrape the MongoDB target 
using HTTPS.
