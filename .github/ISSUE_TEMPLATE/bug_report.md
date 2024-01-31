---
name: Bug report
about: File a report about a problem with the Operator
title: ''
labels: ''
assignees: ''

---
**What did you do to encounter the bug?**
Steps to reproduce the behavior:
1. Go to '...'
2. Click on '....'
3. Scroll down to '....'
4. See error

**What did you expect?**
A clear and concise description of what you expected to happen.

**What happened instead?**
A clear and concise description of what happened instead

**Screenshots**
If applicable, add screenshots to help explain your problem.

**Operator Information**
 - Operator Version
 - MongoDB Image used

**Kubernetes Cluster Information**
 - Distribution
 - Version
 - Image Registry location (quay, or an internal registry)

**Additional context**
Add any other context about the problem here.

If possible, please include:
 - The operator logs
 - Below we are assuming your replicasets database pods is named `mongo`. For instance: 
```                                                                                      
❯ k get pods
NAME      READY   STATUS    RESTARTS   AGE
mongo-0   2/2     Running   0          19h
mongo-1   2/2     Running   0          19h
                                                                                     
❯ k get mdbc
NAME    PHASE     VERSION
mongo   Running   4.4.0
```
 - yaml definitions for your of your MongoDB Deployment
   - `kubectl get mdbc -oyaml`
 - yaml definitions for your kubernetes objects like the statefulset
   - `kubectl get sts -oyaml`
 - The Pod logs:
   - `kubectl logs mongo-0`
 - The agent clusterconfig of the faulty members:
   - `kubectl exec -it mongo-0 -c mongodb-agent -- cat /var/lib/automation/config/cluster-config.json`
 - The agent health status of the faulty members:
   - `kubectl exec -it mongo-0 -c mongodb-agent -- cat /var/log/mongodb-mms-automation/healthstatus/agent-health-status.json`
 - The verbose agent logs of the faulty members:
   - `kubectl exec -it mongo-0 -c mongodb-agent -- cat /var/log/mongodb-mms-automation/automation-agent-verbose.log`
 - You might not have the verbose ones, in that case the non-verbose agent logs works as well:
   - `kubectl exec -it mongo-0 -c mongodb-agent -- cat /var/log/mongodb-mms-automation/automation-agent.log`
