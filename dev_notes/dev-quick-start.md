
#### Prerequisites

* install [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/)
* install python dependencies ```pip install -r requirements.txt```


#### Create a Kind cluster and a local registry
```bash
./scripts/dev/setup_kind_cluster.sh
```

#### set the kind kubernetes context
```bash
export KUBECONFIG=~/.kube/kind
```

#### create the namespace to work in
```bash
kubectl create namespace mongodb

# optionally set it as the default
kubectl config set-context --current --namespace=mongodb
```

#### create a config file for the dev environment
```bash
cat > ~/.community-operator-dev/config.json << EOL
{
    "namespace": "mongodb",
    "repo_url": "localhost:5000",
    "operator_image": "mongodb-kubernetes-operator",
    "e2e_image": "e2e",
    "prestop_hook_image": "prehook",
    "testrunner_image": "test-runner",
    "version_upgrade_hook_image": "community-operator-version-upgrade-post-start-hook"
}
EOL
```

#### build and deploy the operator to the cluster
```bash
python scripts/dev/build_and_deploy_operator.py
```


#### See the operator deployment
```bash
kubectl get pods
```

#### Deploy a Replica Set
```bash
kubectl apply -f deploy/crds/mongodb.com_v1_mongodb_cr.yaml
```
