FROM ubuntu:16.04

ARG agent_version
ARG tools_version

RUN apt-get -qq update \
        && apt-get -y -qq install \
        curl \
        && apt-get upgrade -y -qq \
        && apt-get dist-upgrade -y -qq \
        && rm -rf /var/lib/apt/lists/*

RUN mkdir -p agent \
    && curl --fail --retry 3 --silent https://mciuploads.s3.amazonaws.com/mms-automation/mongodb-mms-build-agent/builds/automation-agent/prod/mongodb-mms-automation-agent-${agent_version}.rhel7_x86_64.tar.gz -o agent/mongodb-agent.tar.gz \
    && tar xfz agent/mongodb-agent.tar.gz \
    && mv mongodb-mms-automation-agent-*/mongodb-mms-automation-agent agent/mongodb-agent \
    && chmod +x agent/mongodb-agent \
    && mkdir -p /var/lib/automation/config \
    && chmod -R +r /var/lib/automation/config \
    && rm agent/mongodb-agent.tar.gz \
    && rm -r mongodb-mms-automation-agent-*
RUN mkdir -p /var/lib/mongodb-mms-automation/probes/ \
    && curl --retry 3 https://readinessprobe.s3-us-west-1.amazonaws.com/readiness -o /var/lib/mongodb-mms-automation/probes/readinessprobe \
    && chmod +x /var/lib/mongodb-mms-automation/probes/readinessprobe \
    && mkdir -p /var/log/mongodb-mms-automation/ \
    && chmod -R +wr /var/log/mongodb-mms-automation/ \
    # ensure that the agent user can write the logs in OpenShift
    && touch /var/log/mongodb-mms-automation/readiness.log \
    && chmod ugo+rw /var/log/mongodb-mms-automation/readiness.log

# Install MongoDB tools. The agent will automatically search the folder and find the binaries.
RUN curl --fail --retry 3 --silent https://downloads.mongodb.org/tools/db/mongodb-database-tools-ubuntu1604-x86_64-${tools_version}.tgz -o mongodb-tools.tgz \
    && tar xfz mongodb-tools.tgz --directory /var/lib/mongodb-mms-automation/ \
    && rm mongodb-tools.tgz

CMD ["agent/mongodb-agent", "-cluster=/var/lib/automation/config/automation-config.json"]
