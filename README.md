# MongoDB Community Kubernetes Operator #

<img align="right" src="https://mongodb-kubernetes-operator.s3.amazonaws.com/img/Leaf-Forest%402x.png">

### v0.6.0 has introduced breaking changes. If you are upgrading from a previous version, follow the upgrade instructions outlined [in the release notes](https://github.com/mongodb/mongodb-kubernetes-operator/releases/tag/v0.6.0)


This is a [Kubernetes Operator](https://coreos.com/operators/) which deploys MongoDB Community into Kubernetes clusters.

If you are a MongoDB Enterprise customer, or need Enterprise features such as Backup, you can use the [MongoDB Enterprise Operator for Kubernetes](https://github.com/mongodb/mongodb-enterprise-kubernetes).

Here is a talk from MongoDB Live 2020 about the Community Operator:
* [Run it in Kubernetes! Community and Enterprise MongoDB in Containers](https://www.youtube.com/watch?v=2Xszdg-4T6A&t=1368s)

## Table of Contents

- [Documentation](#documentation)
- [Supported Features](#supported-features)
  - [Planned Features](#planned-features)
- [Contribute](#contribute)
- [License](#license)

## Documentation

See the [documentation](/docs) to learn how to:

1. [Install or upgrade](/docs/install-upgrade.md) the Operator.
1. [Deploy and configure](/docs/deploy-configure.md) MongoDB resources.
1. [Create a database user](/docs/users.md) with SCRAM authentication.
1. [Secure MongoDB resource connections](/docs/secure.md) using TLS.

*NOTE: [MongoDB Enterprise Kubernetes Operator](https://docs.mongodb.com/kubernetes-operator/master/) docs are for the enterprise operator use case and NOT for the community operator. In addition to the docs mentioned above, you can refer to this [blog post](https://www.mongodb.com/blog/post/run-secure-containerized-mongodb-deployments-using-the-mongo-db-community-kubernetes-oper) as well to learn more about community operator deployment*

## Supported Features

The MongoDB Community Kubernetes Operator supports the following features:

- Create [replica sets](https://docs.mongodb.com/manual/replication/)
- Upgrade and downgrade MongoDB server version
- Scale replica sets up and down
- Read from and write to the replica set while scaling, upgrading, and downgrading. These operations are done in an "always up" manner.
- Report MongoDB server state via the [MongoDBCommunity resource](/config/crd/bases/mongodbcommunity.mongodb.com_mongodbcommunity.yaml) `status` field
- Use any of the available [Docker MongoDB images](https://hub.docker.com/_/mongo/)
- Connect to the replica set from inside the Kubernetes cluster (no external connectivity)
- Secure client-to-server and server-to-server connections with TLS
- Create users with [SCRAM](https://docs.mongodb.com/manual/core/security-scram/) authentication
- Create custom roles

### Planned Features
- Server internal authentication via keyfile

## Contribute

Before you contribute to the MongoDB Community Kubernetes Operator, please read:

- [MongoDB Community Kubernetes Operator Architecture](/docs/architecture.md)
- [Contributing to MongoDB Community Kubernetes Operator](/docs/contributing.md)

Please file issues before filing PRs. For PRs to be accepted, contributors must sign our [CLA](https://www.mongodb.com/legal/contributor-agreement).

Reviewers, please ensure that the CLA has been signed by referring to [the contributors tool](https://contributors.corp.mongodb.com/) (internal link).

## License

Please see the [LICENSE](LICENSE.md) file.
