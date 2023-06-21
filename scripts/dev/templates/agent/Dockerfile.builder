FROM scratch

ADD scripts/dev/templates/agent/LICENSE /data/LICENSE

ARG agent_version
ARG agent_variant
ARG agent_base_url
ARG agent_distro
ARG tools_distro
ARG tools_version
ARG env

ADD ${agent_base_url}/${agent_variant}/mongodb-mms-automation-agent-${agent_version}.${agent_distro}.tar.gz /data/mongodb-agent.tar.gz
ADD https://downloads.mongodb.org/tools/db/mongodb-database-tools-${tools_distro}-${tools_version}.tgz /data/mongodb-tools.tgz


