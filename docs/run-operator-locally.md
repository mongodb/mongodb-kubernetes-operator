# Quick start for building and running the operator locally

This document contains a quickstart guide to build and running (+debugging) the operator locally.
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
- This guide uses IntelliJ as an example. Running any other way should be very similar.
## Goals
- Run the operator locally as a binary (optionally in debug mode) in command line or in an IDE
- Run e2e tests locally 
  - **Note:** 
    - If you plan to run the e2e tests, there are sub-steps that will install the following helm-chart: [operator.yaml](helm-charts/charts/community-operator/templates/operator.yaml)
    - By default, the template chart contains and the operator with `1` replica. This will clash with our local running operator. With this in mind the solution is to set the replicas number to `0` temporarily.

## Steps for Intellij
- Run the make target creating env file `make generate-env-file`, which creates/updates an env file `local-test.env` in .community-operator-dev/
- Use local-test.env file in IntelliJ while running the main of the operator: `cmd/manager/main.go`
![img1](images/intellij-run-env.png)
![img2](images/intellij-run-env-2.png)
