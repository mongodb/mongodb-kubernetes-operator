# Resize PVC Resources #

Resizing the [Persistent Volume Claim (PVC)](https://kubernetes.io/docs/concepts/storage/persistent-volumes/) resources for your Community Kubernetes Operator replica sets using the [StatefulSet](https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/) is [not yet possible](https://github.com/kubernetes/enhancements/pull/3412). Instead, follow these steps to resize the PVC resource for each replica set and recreate the StatefulSet.

1. Enable your storage provisioner to allow volume expansion by setting `allowVolumeExpansion` in the [StorageClass](https://kubernetes.io/docs/concepts/storage/storage-classes/) to `true`. For example:

   ```
   kubectl patch storageclass/<my-storageclass> --type='json' -p='[{"op": "add", "path": "/allowVolumeExpansion", "value": true }]'
   ```

1. If you don't already have a `MongoDBCommunity` resource with custom storage specified, create one. For example:

   ```yaml
   ---
   apiVersion: mongodbcommunity.mongodb.com/v1
   kind: MongoDBCommunity
   metadata:
     name: example-mongodb
   spec:
     members: 3
     type: ReplicaSet
     version: "6.0.5"
     statefulSet:
       spec:
         volumeClaimTemplates:
           - metadata:
               name: data-volume
             spec:
               resources:
                 requests:
                   storage: 50Gi
   ...
   ```

1. Patch the PVC resource for each replica set.

   ```
   kubectl patch pvc/"data-volume-<my-replica-set>-0" -p='{"spec": {"resources": {"requests": {"storage": "100Gi"}}}}'
   kubectl patch pvc/"data-volume-<my-replica-set>-1" -p='{"spec": {"resources": {"requests": {"storage": "100Gi"}}}}'
   kubectl patch pvc/"data-volume-<my-replica-set>-2" -p='{"spec": {"resources": {"requests": {"storage": "100Gi"}}}}'
   ```

1. Scale the Community Kubernetes Operator to `0`.

   ```
   kubectl scale deploy mongodb-kubernetes-operator --replicas=0
   ```

1. Remove the StatefulSet without removing the Pods.

   ```
   kubectl delete sts --cascade=orphan <my-replica-set>
   ```

1. Remove the `MongoDBCommunity` resource without removing the Pods.

   ```
   kubectl delete mdbc --cascade=orphan <my-replica-set>
   ```

1. Scale the Community Kubernetes Operator to `1`.

   ```
   kubectl scale deploy mongodb-kubernetes-operator --replicas=1
   ```

1. Add your new storage specifications to the `MongoDBCommunity` resource. For example:

   ```yaml
   ---
   apiVersion: mongodbcommunity.mongodb.com/v1
   kind: MongoDBCommunity
   metadata:
     name: example-mongodb
   spec:
     members: 3
     type: ReplicaSet
     version: "6.0.5"
     statefulSet:
       spec:
         volumeClaimTemplates:
           - metadata:
               name: data-volume
             spec:
               resources:
                 requests:
                   storage: 100Gi
   ...
   ```

1. Reapply the `MongoDBCommunity` resource. For example:

   ```
   kubectl apply -f PATH/TO/<MongoDBCommunity-resource>.yaml
   ```

1. If your storage provisioner doesn't support online expansion, restart the Pods.

   ```
   kubectl rollout restart sts <my-replica-set>
   ```
