FROM ubuntu:16.04

ARG agent_image=https://s3.amazonaws.com/mongodb-mms-build-agent/releases/prod/mongodb-mms-automation-agent-10.14.3.6344-1.linux_x86_64.tar.gz
ARG agent_version=10.14.3.6344-1

RUN apt-get -qq update \
        && apt-get -y -qq install \
        curl \
        && apt-get upgrade -y -qq \
        && apt-get dist-upgrade -y -qq \
        && rm -rf /var/lib/apt/lists/*

RUN mkdir -p agent \
    && curl --fail --retry 3 --silent $agent_image -o agent/mongodb-agent.tar.gz \
    && tar xfz agent/mongodb-agent.tar.gz \
    && mv mongodb-mms-automation-agent-$agent_version.linux_x86_64/mongodb-mms-automation-agent agent/mongodb-agent \
    && chmod +x agent/mongodb-agent \
    && mkdir -p /var/lib/automation/config \
    && chmod -R +r /var/lib/automation/config

RUN mkdir -p /var/lib/mongodb-mms-automation/probes/ \
#    && curl --retry 3 https://readinessprobe.s3-us-west-1.amazonaws.com/readinessprobe -o /var/lib/mongodb-mms-automation/probes/readinessprobe \
    && curl --retry 3 https://readinessprobe-test.s3-us-west-1.amazonaws.com/readiness -o /var/lib/mongodb-mms-automation/probes/readinessprobe \
    && chmod +x /var/lib/mongodb-mms-automation/probes/readinessprobe \
    && mkdir -p /var/log/mongodb-mms-automation/ \
    && chmod -R +wr /var/log/mongodb-mms-automation/

CMD ["agent/mongodb-agent", "-cluster=/var/lib/automation/config/automation-config.json"]
