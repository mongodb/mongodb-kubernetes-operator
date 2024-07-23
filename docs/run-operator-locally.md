# Quick start for building and running the operator locally

This document contains a quickstart guide to build and running and debugging the operator locally.
Being able to run and build the binary locally can help with faster feedback-cycles.

## Prerequisites
- Follow the general setup to be able to run e2e tests locally with our suite as described here, which includes the usage of [telepresence](https://www.getambassador.io/docs/telepresence/latest/quick-start/):
  - [contributing.md](contributing.md)
  - [build_operator_locally.md](build_operator_locally.md)
  - If above has been configured there should be either:
    - `$HOME/.kube/config`
    - `KUBECONFIG` environment variable pointing at a file
    - **Note**: either of these are necessary to be able to run the operator locally
- Have a folder `.community-operator-dev`
- *Optional - if you want to export the environment variables, you can run the following command*: `source .community-operator-dev/local-test.export.env`. ( These environment variables are generated with the `make generate-env-file`)
## Goals
- Run the operator locally as a binary (optionally in debug mode) in command line or in an IDE
- Run e2e tests locally

## Running The Operator locally
1. Use the dedicated make target which exports the needed environment variables and builds & runs the operator binary.

   Before doing that you need to add 2 more fields to the `config.json` file found in [contributing.md](contributing.md), because the python script looks for them in the file:
    - `mdb_local_operator`: needs to be set to `true`, to allow for the operator to be run locally
    - `kubeconfig`: needs to be set to the path of the `kubeconfig` configuration file, for example `$HOME/.kube/config`
  
   Then you can run the command:
  
    ```sh
       make run
    ```

2.  For debugging one can use the following make target, which uses [dlv](https://github.com/go-delve/delve):

    ```sh
    make debug
    ```

## Running e2e tests with the local operator
- Our [e2e tests](../test/e2e), contains sub-steps that will install the following helm-chart: [operator.yaml](../helm-charts/charts/community-operator/templates/operator.yaml)
- By default, the template chart sets the number of operator replicas to `1`. This will clash with our local running operator. With this in mind the solution is to set the replicas number to `0` temporarily.
- Follow the guide on how to run `e2e` tests as described in our [contributing.md](contributing.md), for instance:

```sh
make e2e-telepresence test=<test-name>
```
