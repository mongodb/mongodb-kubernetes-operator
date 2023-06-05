FROM scratch

ADD scripts/dev/templates/agent/LICENSE /data/LICENSE

ARG agent_version
ARG agent_distro
ARG tools_distro
ARG tools_version
ARG env

ADD https://mciuploads.s3.amazonaws.com/mms-automation/mongodb-mms-build-agent/builds/automation-agent/prod/mongodb-mms-automation-agent-${agent_version}.${agent_distro}.tar.gz /data/mongodb-agent.tar.gz
ADD https://downloads.mongodb.org/tools/db/mongodb-database-tools-${tools_distro}-${tools_version}.tgz /data/mongodb-tools.tgz


https://mciuploads.s3.amazonaws.com/mms-automation/mongodb-mms-build-agent/builds/automation-agent/prod/mongodb-mms-automation-agent-12.0.21.7698-1.linux_x86_64.tar.gz