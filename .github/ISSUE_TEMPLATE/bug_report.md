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
 - `kubectl describe` output
 - yaml definitions for your objects
 - The operator logs
 - Your Custom Resource below assuming the pods are named `mongo-0`
 - The Pod logs:
   - `kubectl logs mongo-0`
 - The agent health status of the faulty members:
   - `kubectl exec -it mongo-0 -c mongodb-agent -- cat /var/log/mongodb-mms-automation/healthstatus/agent-health-status.json`
 - The agent logs of the faulty members:
   - `kubectl exec -it mongo-0 -c mongodb-agent -- cat /var/log/mongodb-mms-automation/automation-agent-verbose.log`
