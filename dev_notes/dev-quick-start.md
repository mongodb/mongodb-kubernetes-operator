
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
    "prestop_hook_image": "prehook",
    "version_upgrade_hook_image": "community-operator-version-upgrade-post-start-hook"
}
EOL
```

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
kubectl apply -f deploy/crds/mongodb.com_v1_mongodbcommunity_cr.yaml
```


### Clean up all resources
```bash
make undeploy
```