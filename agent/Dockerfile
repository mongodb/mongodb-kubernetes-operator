FROM ubuntu:18.04

RUN apt-get -qq update \
        && apt-get -y -qq install \
        curl \
        && apt-get upgrade -y -qq \
        && apt-get dist-upgrade -y -qq \
        && rm -rf /var/lib/apt/lists/*

RUN mkdir -p agent \
    && curl https://s3.amazonaws.com/mciuploads/mms-automation/mongodb-mms-build-agent/builds/patches/5e5546822a60ed501f8c473d/automation-agent/dev/mongodb-mms-automation-agent-10.12.0.6178-1.rhel7_x86_64.tar.gz -o agent/mongodb-agent.tar.gz \
    && tar xfz agent/mongodb-agent.tar.gz \
    && mv mongodb-mms-automation-agent-10.12.0.6178-1.rhel7_x86_64/mongodb-mms-automation-agent agent/mongodb-agent \
    && chmod +x agent/mongodb-agent \
    && mkdir -p /var/lib/automation/config \
    && chmod -R +r /var/lib/automation/config

CMD ["agent/mongodb-agent", "-cluster=/var/lib/automation/config/automation-config.json"]
