# MongoDB Community Kubernetes Operator #

<img align="right" src="https://mongodb-kubernetes-operator.s3.amazonaws.com/img/Leaf-Forest%402x.png">

This is a [Kubernetes Operator](https://coreos.com/operators/) which deploys MongoDB Community into Kubernetes clusters.

If you are a MongoDB Enterprise customer, or need Enterprise features such as Backup, you can use the [MongoDB Enterprise Operator for Kubernetes](https://github.com/mongodb/mongodb-enterprise-kubernetes).

## Table of Contents

- [Documentation](#documentation)
- [Supported Features](#supported-features)
- [Contribute](#contribute)
- [License](#license)

## Documentation

See the [`/docs`](docs) directory to view documentation on how to:

1. [Install or upgrade](docs/install-upgrade.md) the Operator.
1. [Deploy and configure](docs/deploy-configure.md) MongoDB resources.
1. [Create a database user] with SCRAM-SHA authentication.
1. [Secure](docs/secure.md) MongoDB resources.

## Supported Features

The MongoDB Community Kubernetes Operator supports the following features:

- MongoDB Topology: [replica sets](https://docs.mongodb.com/manual/replication/)
- Upgrading and downgrading MongoDB server version
- Scaling replica sets up and down
- Reading from and writing to the replica set while scaling, upgrading, and downgrading. These operations are done in an "always up" manner.
- Reporting of MongoDB server state via the [MongoDB resource](/deploy/crds/mongodb.com_mongodb_crd.yaml) `status` field
- Use of any of the available [Docker MongoDB images](https://hub.docker.com/_/mongo/)
- Clients inside the Kubernetes cluster can connect to the replica set (no external connectivity)
- TLS support for client-to-server and server-to-server communication
- Creating users with SCRAM-SHA authentication

### Planned Features
- Server internal authentication via keyfile

## Contribute

Before you contribute to the MongoDB Community Kubernetes Operator, please read:

- [MongoDB Community Kubernetes Operator Architecture](docs/architecture.md)
- [Contributing to MongoDB Community Kubernetes Operator](docs/contributing.md)

Please file issues before filing PRs. For PRs to be accepted, contributors must sign our [CLA](https://www.mongodb.com/legal/contributor-agreement).

Reviewers, please ensure that the CLA has been signed by referring to [the contributors tool](https://contributors.corp.mongodb.com/) (internal link).

## License

Please see the [LICENSE](LICENSE.md) file.
