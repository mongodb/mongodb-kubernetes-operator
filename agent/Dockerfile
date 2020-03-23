FROM ubuntu:16.04

# https://jira.mongodb.org/browse/CLOUDP-58488
# Adds -noDaemonize option
ARG agent_image=https://s3.amazonaws.com/mciuploads/mms-automation/mongodb-mms-build-agent/builds/patches/5e600292d6d80a561311da80/automation-agent/dev/mongodb-mms-automation-agent-10.12.0.6191-1.rhel7_x86_64.tar.gz
ARG agent_version=10.12.0.6191

RUN apt-get -qq update \
        && apt-get -y -qq install \
        curl \
        && apt-get upgrade -y -qq \
        && apt-get dist-upgrade -y -qq \
        && rm -rf /var/lib/apt/lists/*

RUN mkdir -p agent \
    && curl $agent_image -o agent/mongodb-agent.tar.gz \
    && tar xfz agent/mongodb-agent.tar.gz \
    && mv mongodb-mms-automation-agent-$agent_version-1.rhel7_x86_64/mongodb-mms-automation-agent agent/mongodb-agent \
    && chmod +x agent/mongodb-agent \
    && mkdir -p /var/lib/automation/config \
    && chmod -R +r /var/lib/automation/config

# TODO: this mongo client is used for testing purposes and should be removed in the future
RUN curl -LO http://downloads.mongodb.org/linux/mongodb-linux-x86_64-ubuntu1604-4.0.6.tgz && \
    tar xfz mongodb-linux-x86_64-ubuntu1604-4.0.6.tgz && \
    mv mongodb-linux-x86_64-ubuntu1604-4.0.6/bin/mongo /usr/bin && \
    rm -rf mongodb-linux-x86_64-ubuntu1604-4.0.6.tgz mongodb-linux-x86_64-ubuntu1604-4.0.6


RUN mkdir -p /var/lib/mongodb-mms-automation/probes/ \
    && curl --retry 3 https://readinessprobe.s3-us-west-1.amazonaws.com/readinessprobe -o /var/lib/mongodb-mms-automation/probes/readinessprobe \
    && chmod +x /var/lib/mongodb-mms-automation/probes/readinessprobe \
    && mkdir -p /var/log/mongodb-mms-automation/ \
    && chmod -R +wr /var/log/mongodb-mms-automation/

CMD ["agent/mongodb-agent", "-cluster=/var/lib/automation/config/automation-config.json"]
