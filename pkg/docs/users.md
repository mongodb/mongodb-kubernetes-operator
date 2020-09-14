# Create a Database User #

You can create database users through the [MongoDB CRD](deploy/crds/mongodb.com_v1_mongodb_scram_cr.yaml).

SCRAM authentication is always enabled.

## Table of Contents

- [Create a Database User with SCRAM Authentication](#create-a-database-user-with-scram-authentication)
  - [Create a User Secret](#create-a-user-secret)
  - [Modify the MongoDB CRD](#modify-the-mongodb-crd)

## Create a Database User with SCRAM Authentication

### Create a User Secret

1. Copy the following example secret.

     ```
     ---
     apiVersion: v1
     kind: Secret
     metadata:
       name: <db-user-secret>  # corresponds to spec.users.passwordSecretRef.name
     type: Opaque
     stringData:
       password: <my-plain-text-password> # corresponds to spec.users.passwordSecretRef.key
     ...
     ```
1. Update `metadata.name` with the name of the secret and `stringData.password` with the user's password.
1. Save the secret with a `.yaml` file extension.
1. Apply the secret in Kubernetes:
   ```
   kubectl apply -f <db-user-secret>.yaml
   ```

### Modify the MongoDB CRD

1. Add the following fields to the MongoDB resource definition:

   | *Key* | *Type* | *Description* | *Required?* |
   |----|----|----|----|
   | spec.users | array of objects | Configures database users for this deployment. | Yes |
   | spec.users.name | string | Username of the database user. | Yes |
   | spec.users.db | string | Database that the user authenticates against. Defaults to `admin`. | No |
   | spec.users.passwordSecretRef.name | string | Name of the secret that contains the user's plain text password. | |Yes|
   | spec.users.passwordSecretRef.key | string| Key in the secret that corresponds to the value of the user's password. Defaults to `password`. | No |
   | spec.users.roles | array of objects | Configures roles assigned to the user. | Yes |
   | spec.users.roles.role.name | string | Name of the role. Valid values are [built-in roles](https://docs.mongodb.com/manual/reference/built-in-roles/#built-in-roles). | Yes |
   | spec.users.roles.role.db | string | Database that the role applies to. | Yes |

   ```
   ---
   apiVersion: mongodb.com/v1
   kind: MongoDB
   metadata:
     name: example-scram-mongodb
   spec:
     members: 3
     type: ReplicaSet
     version: "4.2.6"
     security:
       authentication:
         modes: ["SCRAM"]
     users:
       - name: <username>
         db: <authentication-database>
         passwordSecretRef: 
           name: <db-user-secret>
         roles:
           - name: <role-1>
             db: <role-1-database>
           - name: <role-2>
             db: <role-2-database>
   ...
   ```
1. Save the file.
1. Apply the updated MongoDB resource definition:

   ```
   kubectl apply -f <crd>.yaml --namespace <my-namespace>
   ```
   The Operator generates SCRAM credentials for the new user from the password secret. 
1. Once the resource is running, delete the password secret.
   ```
   kubectl delete secret <db-user-secret> --namespace <my-namespace>
   ```
