# Contributing to MongoDB Kubernetes Operator

First you need to get familiar with the [Architecture guide](architecture.md), which explains
from a high perspective how everything works together.

After our experience building the [Enterprise MongoDB Kubernetes
Operator](https://github.com/mongodb/mongodb-enterprise-operator), we have
realized that is is very important to have a clean environment to work, and as such we have
adopted a strategy that makes it easier for everyone to contribute.

This strategy is based on using
[`envtest`](https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/envtest) for setting up the tests
and `go test` for running the tests, and making the test-runner itself run as a Kubernetes Pod. This
makes it easier to run the tests in environments with access to a Kubernetes
cluster with no go toolchain installed locally, making it easier to reproduce
our local working environments in CI/CD systems.

# High-Perspective Architecture

The operator itself consists of 1 image, that has all the operational logic to deploy and
maintain the MongoDB resources in your cluster.

The operator deploys MongoDB in Pods (via a higher-level resource, a
StatefulSet), on each Pod there will be multiple images coexisting during the
lifetime of the MongoDB server.

* Agent image: This image includes a binary provided by MongoDB that handles
the local operation of a MongoDB server given a series of configurations
provided by the operator. The configuration exists as a ConfigMap that's created
by the operator and mounted in the Agent's Pod.

* MongoDB image: Docker image that includes the MongoDB server.

* Version upgrade post-start hook image: This image includes a binary that helps orchestrate the
  restarts of the MongoDB Replica Set members, in particular, when dealing with
  version upgrades, which requires a very precise set of operations to allow for
  seamless upgrades and downgrades, with no downtime.

Each Pod holds a member of a Replica Set, and each Pod has different components,
each one of them in charge of some part of the lifecycle of the MongoDB database.

# Developing locally

The operator is built using `golang`. We use a simple
json file that describe some local options that you need to set for the testing environment
to be able to run properly. Create a json file with the following content:

```json
{
    "namespace": "default",
    "repo_url": "localhost:5000",
    "operator_image": "mongodb-kubernetes-operator",
    "e2e_image": "e2e",
    "version_upgrade_hook_image": "version_upgrade_hook",
    "testrunner_image": "test-runner"
}
```

The `namespace` attribute sets the Kubernetes namespace to be used for running
your tests. The `repo_url` sets the Docker registry. In my case I have a
`local-config.json` file in the root of this repo. For the e2e tests to pick
this file, set the `MONGODB_COMMUNITY_CONFIG` env variable to the absolute path
of this file.

Please see [here](./build_operator_locally.md) to see how to build and deploy the operator locally.

## Configure Docker registry

The build process consist in multiple Docker images being built, you need to specify
where you want the locally build images to be pushed. The Docker registry needs to be
accessible from your Kubernetes cluster.

## Test Namespace

You can change the namespace used for tests, if you are using `Kind`, for
instance, you can leave this as `default`.

## Python Environment

The test runner is a Python script, in order to use it a virtualenv needs to be
created. The dependencies of the Python environment are described, as usual, in
a `requirements.txt` file:

```sh
python -m venv venv
source venv/bin/activate
python -m pip install -r requirements.txt
```

# Running Unit tests

Unit tests should be run from the root of the project with:

```sh
go test ./pkg/...
```

# Running E2E Tests

## Running an E2E test

We have built a simple mechanism to run E2E tests on your cluster using a runner
that deploys a series of Kubernetes objects, runs them, and awaits for their
completion. If the objects complete with a Success status, it means that the
tests were run successfully.

The available tests can be found in the `tests/e2e` directory, at the time of this
writting we have:

```sh
$ ls -l test/e2e
replica_set
replica_set_change_version
replica_set_readiness_probe
replica_set_scale
...
```

The tests should run individually using the runner like this:

```sh
# python scripts/dev/e2e.py --test <test-name>
# for example
python scripts/dev/e2e.py --test replica_set
```

This will run the `replica_set` E2E test which is a simple test that installs a
MongoDB Replica Set and asserts that the deployed server can be connected to.


The python script has several flags to control its behaviour, please run

```sh
python scripts/dev/e2e.py --help
```

to get a list.

## Troubleshooting
When you run a test locally, if the `e2e-test` pod is present, you will have to
first manually delete it; failing to do so will cause the `test-runner` pod to fail.

# Writing new E2E tests

You can start with `replica_set` test as an starting point to write a new test.
The tests are written using `envtest` and they are run using `go test`.

Adding a new test is as easy as to create a new directory in `test/e2e` with the
new E2E test, and to run them:

```sh
python scripts/dev/e2e.py --test <new-test>
```

# Before Committing your code

## Set up pre-commit hooks
To set up the pre-commit hooks, please create symbolic links from the provided [hooks](https://github.com/mongodb/mongodb-kubernetes-operator/tree/master/scripts/git-hooks):

* Navigate to your `.git/hooks` directory:

	`cd .git/hooks`

* Create a symlink for every file in the `scripts/git-hooks` directory:

	`ln -s -f  ../../scripts/git-hooks/* .`


## Please make sure you sign our Contributor Agreement
You can find it [here](https://www.mongodb.com/legal/contributor-agreement). This will be
required when creating a PR against this repo!
