kubectl exec -it ${1} -c mongodb-agent cat /var/log/mongodb-mms-automation/agent-health-status.json | jq
