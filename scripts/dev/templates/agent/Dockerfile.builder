FROM curlimages/curl:7.76.1 as builder

ARG agent_version
ARG agent_distro
ARG tools_distro
ARG tools_version

USER root

RUN mkdir -p data && \
 curl --fail --retry 3 --silent https://mciuploads.s3.amazonaws.com/mms-automation/mongodb-mms-build-agent/builds/automation-agent/prod/mongodb-mms-automation-agent-${agent_version}.${agent_distro}.tar.gz -o data/mongodb-agent.tar.gz && \
 curl --fail --retry 3 --silent https://downloads.mongodb.org/tools/db/mongodb-database-tools-${tools_distro}-${tools_version}.tgz -o data/mongodb-tools.tgz

FROM scratch

COPY --from=builder data/mongodb-agent.tar.gz /data/
COPY --from=builder data/mongodb-tools.tgz /data/
ADD agent/LICENSE /data/licenses
