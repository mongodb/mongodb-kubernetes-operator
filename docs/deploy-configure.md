# Deploy and Configure a MongoDBCommunity Resource #

The [`/config/samples`](../config/samples) directory contains example MongoDBCommunity resources that you can modify and deploy.

## Table of Contents

- [Deploy a Replica Set](#deploy-a-replica-set)
- [Scale a Replica Set](#scale-a-replica-set)
- [Add Arbiters to a Replica Set](#add-arbiters-to-a-replica-set)
- [Upgrade your MongoDBCommunity Resource Version and Feature Compatibility Version](#upgrade-your-mongodbcommunity-resource-version-and-feature-compatibility-version)
  - [Example](#example)
- [Deploy Replica Sets on OpenShift](#deploy-replica-sets-on-openshift)
- [Define a Custom Database Role](#define-a-custom-database-role)
- [Specify Non-Default Values for Readiness Probe](#specify-non-default-values-for-readiness-probe)
  - [When to specify custom values for the Readiness Probe](#when-to-specify-custom-values-for-the-readiness-probe)

## Deploy a Replica Set

**Warning:** When you delete MongoDB resources, persistent volumes remain
to help ensure that no unintended data loss occurs. If you create a new 
MongoDB resource with the same name and persistent volumes, the 
pre-existing data might cause issues if the new MongoDB resources have a
different topology than the previous ones.

To deploy your first replica set:

1. Replace `<your-password-here>` in [config/samples/mongodb.com_v1_mongodbcommunity_cr.yaml](../config/samples/mongodb.com_v1_mongodbcommunity_cr.yaml) to the password you wish to use.
2. Invoke the following `kubectl` command:
   ```
   kubectl apply -f config/samples/mongodb.com_v1_mongodbcommunity_cr.yaml --namespace <my-namespace>
   ```
3. Verify that the MongoDBCommunity resource deployed:
   ```
   kubectl get mongodbcommunity --namespace <my-namespace>
   ```

4. The Community Kubernetes Operator creates secrets that contains users' connection strings and credentials.

   The secrets follow this naming convention: `<metadata.name>-<auth-db>-<username>`, where:

   | Variable | Description | Value in Sample |
   |----|----|----|
   | `<metadata.name>` | Name of the MongoDB database resource. | `example-mongodb` |
   | `<auth-db>` | [Authentication database](https://www.mongodb.com/docs/manual/core/security-users/#std-label-user-authentication-database) where you defined the database user. | `admin` |
   | `<username>` | Username of the database user. | `my-user` |

   **NOTE**: Alternatively, you can specify an optional
   `users[i].connectionStringSecretName` field in the
   ``MongoDBCommunity`` custom resource to specify
   the name of the connection string secret that the
   Community Kubernetes Operator creates.

   Update the variables in the following command, then run it to retrieve a user's connection strings to the replica set from the secret:

   **NOTE**: The following command requires [jq](https://stedolan.github.io/jq/) version 1.6 or higher.</br></br>

   ```sh
   kubectl get secret <connection-string-secret-name> -n <my-namespace> \
   -o json | jq -r '.data | with_entries(.value |= @base64d)'
   ```

   The command returns the replica set's standard and DNS seed list [connection strings](https://www.mongodb.com/docs/manual/reference/connection-string/#connection-string-formats) in addition to the user's name and password:

   ```json
   {
     "connectionString.standard": "mongodb://<username>:<password>@example-mongodb-0.example-mongodb-svc.mongodb.svc.cluster.local:27017,example-mongodb-1.example-mongodb-svc.mongodb.svc.cluster.local:27017,example-mongodb-2.example-mongodb-svc.mongodb.svc.cluster.local:27017/admin?ssl=true",
     "connectionString.standardSrv": "mongodb+srv://<username>:<password>@example-mongodb-svc.mongodb.svc.cluster.local/admin?ssl=true",
     "password": "<password>",
     "username": "<username>"
   }
   ```

   **NOTE**: The Community Kubernetes Operator sets the [`ssl` connection option](https://www.mongodb.com/docs/manual/reference/connection-string/#connection-options) to `true` if you [Secure MongoDBCommunity Resource Connections using TLS](secure.md#secure-mongodbcommunity-resource-connections-using-tls).</br></br>

   You can use the connection strings in this secret in your application:

   ```yaml
   containers:
    - name: test-app
      env:
       - name: "CONNECTION_STRING"
         valueFrom:
           secretKeyRef:
             name: <metadata.name>-<auth-db>-<username>
             key: connectionString.standardSrv

5. Connect to one of your application's pods in the Kubernetes cluster:

   **NOTE**: You can access your replica set only from a pod in the same Kubernetes cluster. You can't access your replica set from outside of the Kubernetes cluster.

   ```
   kubectl -n <my-namespace> exec --stdin --tty <your-application-pod> -- /bin/bash
   ```

   When you connect to your application pod, a shell prompt appears for your application's container:

   ```
   user@app:~$
   ```

6. Use one of the connection strings returned in step 4 to connect to the replica set. The following example uses [`mongosh`](https://www.mongodb.com/docs/mongodb-shell/) to connect to a replica set:

   ```
   mongosh "mongodb+srv://<username>:<password>@example-mongodb-svc.mongodb.svc.cluster.local/admin?ssl=true"
   ```

## Scale a Replica Set

You can scale up (increase) or scale down (decrease) the number of
members in a replica set.

Consider the following example MongoDBCommunity resource definition:

```yaml
apiVersion: mongodbcommunity.mongodb.com/v1
kind: MongoDBCommunity
metadata:
  name: example-mongodb
spec:
  members: 3
  type: ReplicaSet
  version: "4.2.7"
```

To scale a replica set:

1. Edit the resource definition.

   Update `members` to the number of members that you want the replica set to have.

   ```yaml
   apiVersion: mongodbcommunity.mongodb.com/v1
   kind: MongoDBCommunity
   metadata:
     name: example-mongodb
   spec:
     members: 5
     type: ReplicaSet
     version: "4.2.7"
   ```

2. Reapply the configuration to Kubernetes:
   ```
   kubectl apply -f <example>.yaml --namespace <my-namespace>
   ```

   **NOTE**: When you scale down a MongoDBCommunity resource, the Community Operator
   might take several minutes to remove the StatefulSet replicas for the
   members that you remove from the replica set.

## Add Arbiters to a Replica Set

To add [arbiters](https://www.mongodb.com/docs/manual/core/replica-set-arbiter/) to
your replica set, add the `spec.arbiters` field to your MongoDBCommunity
resource definition. This attribute configures the absolute amount of arbiters
in this Replica Set, this is, the amount of `mongod` instances will be
`spec.members` + `spec.arbiters`.

The value of the `spec.arbiters` field must be:

- a positive integer, and
- less than the value of the `spec.members` field.

**NOTE**: At least one replica set member must not be an arbiter.

Consider the following MongoDBCommunity resource definition example, with a PSS
(Primary-Secondary-Secondary) configuration:

```yaml
apiVersion: mongodbcommunity.mongodb.com/v1
kind: MongoDBCommunity
metadata:
  name: example-mongodb
spec:
  type: ReplicaSet
  members: 3
  version: "4.2.7"
```

To add arbiters:

1. Edit the resource definition.

   Add the `spec.arbiters` field and assign its value to the number of arbiters that you want the replica set to have.

   ```yaml
   apiVersion: mongodbcommunity.mongodb.com/v1
   kind: MongoDBCommunity
   metadata:
     name: example-mongodb
   spec:
     type: ReplicaSet
     members: 3
     arbiters: 1
     version: "4.4.13"
   ```

2. Reapply the configuration to Kubernetes:
   ```
   kubectl apply -f <example>.yaml --namespace <my-namespace>
   ```

The resulting Replica Set has a PSSA (Primary-Secondary-Secondary-Arbiter)
configuration.

## Upgrade your MongoDBCommunity Resource Version and Feature Compatibility Version

You can upgrade the major, minor, and/or feature compatibility versions of your MongoDBCommunity resource. These settings are configured in your resource definition YAML file.

- To upgrade your resource's major and/or minor versions, set the `spec.version` setting to the desired MongoDB version. Make sure to specify a full image tag, such as `5.0.3`. Setting the `spec.version` to loosely-defined tags such as `5.0` is not currently supported.

- To modify your resource's [feature compatibility version](https://www.mongodb.com/docs/manual/reference/command/setFeatureCompatibilityVersion/), set the `spec.featureCompatibilityVersion` setting to the desired version.

If you update `spec.version` to a later version, consider setting `spec.featureCompatibilityVersion` to the current working MongoDB version to give yourself the option to downgrade if necessary. To learn more about feature compatibility, see [`setFeatureCompatibilityVersion`](https://www.mongodb.com/docs/manual/reference/command/setFeatureCompatibilityVersion/) in the MongoDB Manual.

### Example

Consider the following example MongoDBCommunity resource definition:

```yaml
apiVersion: mongodbcommunity.mongodb.com/v1
kind: MongoDBCommunity
metadata:
  name: example-mongodb
spec:
  members: 3
  type: ReplicaSet
  version: "4.0.6"
```
To upgrade this resource from `4.0.6` to `4.2.7`:

1. Edit the resource definition.

   a. Update `spec.version` to `4.2.7`.

   b. Update `spec.featureCompatibilityVersion` to `4.0`.

   ```yaml
   apiVersion: mongodbcommunity.mongodb.com/v1
   kind: MongoDBCommunity
   metadata:
     name: example-mongodb
   spec:
     members: 3
     type: ReplicaSet
     version: "4.2.7"
     featureCompatibilityVersion: "4.0"
   ```

   **NOTE:** Setting `featureCompatibilityVersion` to `4.0` disables [4.2 features incompatible with MongoDB 4.0](https://www.mongodb.com/docs/manual/release-notes/4.2-compatibility/#compatibility-enabled).

2. Reapply the configuration to Kubernetes:
   ```
   kubectl apply -f <example>.yaml --namespace <my-namespace>
   ```

## Deploy Replica Sets on OpenShift

To deploy the operator on OpenShift you will have to provide the environment variable `MANAGED_SECURITY_CONTEXT` set to `true` for the operator deployment.

See [here](../config/samples/mongodb.com_v1_mongodbcommunity_openshift_cr.yaml) for
an example of how to provide the required configuration for a MongoDB
replica set.

See [here](../deploy/openshift/operator_openshift.yaml) for an example of how to configure the Operator deployment.

## Define a Custom Database Role

You can define [custom roles](https://www.mongodb.com/docs/manual/core/security-user-defined-roles/) to give you fine-grained access control over your MongoDB database resource.

  **NOTE**: Custom roles are scoped to a single MongoDB database resource.

To define a custom role:

1. Add the following fields to the MongoDBCommunity resource definition:

   | Key | Type | Description | Required? |
   |----|----|----|----|
   | `spec.security.authentication.ignoreUnknownUsers` | boolean | Flag that indicates whether you can add users that don't exist in the `MongoDBCommunity` resource. If omitted, defaults to `true`. | No |
   | `spec.security.roles` | array | Array that defines [custom roles](https://www.mongodb.com/docs/manual/core/security-user-defined-roles/) roles that give you fine-grained access control over your MongoDB deployment. | Yes |
   | `spec.security.roles.role` | string | Name of the custom role. | Yes |
   | `spec.security.roles.db` | string | Database in which you want to store the user-defined role. | Yes |
   | `spec.security.roles.authenticationRestrictions` | array | Array that defines the IP address from which and to which users assigned this role can connect. | No |
   | `spec.security.roles.authenticationRestrictions.clientSource` | array | Array of IP addresses or CIDR blocks from which users assigned this role can connect. <br><br> MongoDB servers reject connection requests from users with this role if the requests come from a client that is not present in this array. | No |
   | `spec.security.roles.authenticationRestrictions.serverAddress` | array | Array of IP addresses or CIDR blocks to which users assigned this role can connect. <br><br> MongoDB servers reject connection requests from users with this role if the client requests to connect to a server that is not present in this array. | No |
   | `spec.security.roles.privileges` | array | List of actions that users granted this role can perform. For a list of accepted values, see [Privilege Actions](https://www.mongodb.com/docs/manual/reference/privilege-actions/#database-management-actions) in the MongoDB Manual for the MongoDB versions you deploy with the Kubernetes Operator. | Yes |
   | `spec.security.roles.privileges.actions` | array | Name of the role. Valid values are [built-in roles](https://www.mongodb.com/docs/manual/reference/built-in-roles/#built-in-roles). | Yes |
   | `spec.security.roles.privileges.resource.database`| string | Database for which the privilege `spec.security.roles.privileges.actions` apply. An empty string (`""`) indicates that the privilege actions apply to all databases. <br><br> If you provide a value for this setting, you must also provide a value for `spec.security.roles.privileges.resource.collection`. | Conditional |
   | `spec.security.roles.privileges.resource.collection`| string | Collection for which the privilege `spec.security.roles.privileges.actions` apply. An empty string (`""`) indicates that the privilege actions apply to all of the database's collections.<br><br> If you provide a value for this setting, you must also provide a value for `spec.security.roles.privileges.resource.database`. | Conditional |
   | `spec.security.roles.privileges.resource.cluster`| string | Flag that indicates that the privilege `spec.security.roles.privileges.actions` apply to all databases and collections in the MongoDB deployment. If omitted, defaults to `false`.<br><br> If set to `true`, do not provide values for `spec.security.roles.privileges.resource.database` and `spec.security.roles.privileges.resource.collection`. | Conditional |
   | `spec.security.roles.roles`| array | An array of roles from which this role inherits privileges. <br><br> You must include the roles field. Use an empty array (`[]`) to specify no roles to inherit from. | Yes |
   | `spec.security.roles.roles.role` | string | Name of the role to inherit from. | Conditional |
   | `spec.security.roles.roles.database` | string | Name of database that contains the role to inherit from. | Conditional |

   ```yaml
   ---
   apiVersion: mongodbcommunity.mongodb.com/v1
   kind: MongoDBCommunity
   metadata:
     name: custom-role-mongodb
   spec:
     members: 3
     type: ReplicaSet
     version: "4.2.6"
     security:
       authentication:
         modes: ["SCRAM"]
       roles: # custom roles are defined here
         - role: testRole
           db: admin
           privileges:
             - resource:
                 db: "test"
                 collection: "" # an empty string indicates any collection
               actions:
                 - find
           roles: []
     users:
       - name: my-user
         db: admin
         passwordSecretRef: # a reference to the secret that will be used to generate the user's password
           name: my-user-password
         roles:
           - name: clusterAdmin
             db: admin
           - name: userAdminAnyDatabase
             db: admin
           - name: testRole # apply the custom role to the user
             db: admin
         scramCredentialsSecretName: my-scram
   ```

2. Save the file.
3. Apply the updated MongoDBCommunity resource definition:

   ```
   kubectl apply -f <mongodb-crd>.yaml --namespace <my-namespace>
   ```


## Specify Non-Default Values for Readiness Probe

Under some circumstances it might be necessary to set your own custom values for
the `ReadinessProbe` used by the MongoDB Community Operator. To do so, you
should use the `statefulSet` attribute in `resource.spec`, as in the following
provided example [yaml
file](../config/samples/mongodb.com_v1_mongodbcommunity_readiness_probe_values.yaml).
Only those attributes passed will be set, for instance, given the following structure:

```yaml
spec:
  statefulSet:
    spec:
      template:
        spec:
          containers:
            - name: mongodb-agent
              readinessProbe:
                failureThreshold: 40
                initialDelaySeconds: 5
```

*Only* the values of `failureThreshold` and `initialDelaySeconds` will be set to
their custom, specified values. The rest of the attributes will be set to their
default values.

*Please note that these are the actual values set by the Operator for our
MongoDB Custom Resources.*

### When to specify custom values for the Readiness Probe

In some cases, for instance, with a less than optimal download speed from the
image registry, it could be necessary for the Operator to tolerate a Pod that
has taken longer than expected to restart or upgrade to a different version of
MongoDB. In these cases we want the Kubernetes API to wait a little longer
before giving up, we could increase the value of `failureThreshold` to `60`.

In other cases, if the Kubernetes API is slower than usual, we would increase
the value of `periodSeconds` to `20`, so the Kubernetes API will do half of the
requests it normally does (default value for `periodSeconds` is `10`).

*Please note that these are referential values only!*

### Operator Configurations

#### Modify cluster domain for MongoDB service objects

To configure the cluster domain for the MongoDB service object, i.e use a domain other than the default `cluster.local` you can specify it as an environment variable in the operator deployment under `CLUSTER_DOMAIN` key.

For ex:
```yaml
env:
  - name: CLUSTER_DOMAIN
    value: $CUSTOM_DOMAIN
```