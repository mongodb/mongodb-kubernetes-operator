# Configure Logging in MongoDB Community

This section describes the components which are logging either to a file or stdout,
how to configure them and what their defaults are.

## MongoDB Processes
### Configuration
The exposed CRD options can be seen [in the crd yaml](https://github.com/mongodb/mongodb-kubernetes-operator/blob/74d13f189566574b862e5670b366b61ec5b65923/config/crd/bases/mongodbcommunity.mongodb.com_mongodbcommunity.yaml#L105-L117).
Additionally, more information regarding configuring systemLog can be found [in the official documentation of systemLog](https://www.mongodb.com/docs/manual/reference/configuration-options/#core-options)].
`spec.agent.systemLog.destination` configures the logging destination of the mongod process.
### Default Values
By default, MongoDB sends all log output to standard output. 

## MongoDB Agent
### Configuration
`spec.agent.logFile` can be used to configure the output file of the mongoDB agent logging. 
The agent will log to standard output with the following setting: `/dev/stdout`.
### Default Values
By default, the MongoDB agent logs to `/var/log/mongodb-mms-automation/automation-agent.log`
 
## ReadinessProbe 
### Configuration & Default Values
The readinessProbe can be configured via Environment variables. 
Below is a table with each environment variable, its explanation and its default value.

| Environment Variable            | Explanation                                                             | Default Value                                 |
|---------------------------------|-------------------------------------------------------------------------|-----------------------------------------------|
| READINESS_PROBE_LOGGER_BACKUPS  | maximum number of old log files to retain                               | 5                                             |
| READINESS_PROBE_LOGGER_MAX_SIZE | maximum size in megabytes                                               | 5                                             |
| READINESS_PROBE_LOGGER_MAX_AGE  | maximum number of days to retain old log files                          | none                                          |
| READINESS_PROBE_LOGGER_COMPRESS | if the rotated log files should be compressed                           | false                                         |
| MDB_WITH_AGENT_FILE_LOGGING     | whether we should also log to stdout (which shows in kubectl describe)  | true                                          |
| LOG_FILE_PATH                   | path of the logfile of the readinessProbe.                              | /var/log/mongodb-mms-automation/readiness.log |