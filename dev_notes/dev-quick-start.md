
#### Prerequisites

* Install [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/)
* Install [jq](https://stedolan.github.io/jq/download/) 
* Optionally install [kind](https://kind.sigs.k8s.io/docs/user/quick-start/#installation) if you want to use a local cluster.

* create a python virtual environment

```bash
python3 -m venv /path/to/new/virtual/environment
source path/to/new/virtual/environment/bin/activate
```

* install python dependencies 
```
pip install -r requirements.txt

# Note: sonar requires access to the 10gen repo and is used for the release pipeline
pip install git+ssh://git@github.com/10gen/sonar.git@0.0.10
```

#### Create a Kind cluster and a local registry
```bash
./scripts/dev/setup_kind_cluster.sh
```

#### set the kind kubernetes context
```bash
export KUBECONFIG=~/.kube/kind
```

#### Get kind credentials
```bash
kind export kubeconfig

# check it worked by running:
kubectl cluster-info --context kind-kind --kubeconfig $KUBECONFIG
```


#### (Optional) Create a non-default namespace to work in
```bash
kubectl create namespace mongodb

# optionally set it as the default
kubectl config set-context --current --namespace=mongodb
```

#### create a config file for the dev environment
```bash
cat > ~/.community-operator-dev/config.json << EOL
{
    "namespace": "default",
    "repo_url": "localhost:5000",
    "operator_image": "mongodb-kubernetes-operator",
    "e2e_image": "e2e",
    "testrunner_image": "test-runner",
    "version_upgrade_hook_image": "community-operator-version-upgrade-post-start-hook",
    "readiness_probe_image": "mongodb-kubernetes-readinessprobe"
    "version_upgrade_hook_image": "community-operator-version-upgrade-post-start-hook"
}
EOL
```

More details about the config options can be found [here](./config-options.md)

#### build and deploy the operator to the cluster
```bash
make docker-build docker-push deploy
```


#### See the operator deployment
```bash
kubectl get pods
```

#### Deploy a Replica Set
```bash
make deploy-dev-quick-start-rs
```

#### See the deployed replica set
```bash
kubectl get pods

NAME                                           READY   STATUS    RESTARTS   AGE
mongodb-kubernetes-operator-5568d769b8-smt4h   1/1     Running   0          4m12s
quick-start-rs-0                               2/2     Running   0          2m49s
quick-start-rs-1                               2/2     Running   0          2m5s
quick-start-rs-2                               2/2     Running   0          87s


kubectl get statefulset quick-start-rs

NAME             READY   AGE
quick-start-rs   3/3     3m10s
```

### Clean up all resources
```bash
make undeploy
```

### Running Tests

Follow the tests in [contributing.md](../docs/contributing.md) to run e2e tests.