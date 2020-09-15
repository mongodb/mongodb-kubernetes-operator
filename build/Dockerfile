FROM ubuntu:16.04

ENV OPERATOR=/usr/local/bin/mongodb-kubernetes-operator \
    USER_UID=1001 \
    USER_NAME=mongodb-kubernetes-operator

RUN apt-get update  && \
    apt-get install -y curl

ENV manifest_version=4.4
RUN mkdir -p /content/ \
        && chmod -R +r /content/ \
        && curl --fail --retry 3 -o /usr/local/version_manifest.json "https://opsmanager.mongodb.com/static/version_manifest/${manifest_version}.json"

# install operator binary
COPY build/_output/bin/mongodb-kubernetes-operator ${OPERATOR}

COPY build/bin /usr/local/bin
RUN  /usr/local/bin/user_setup

ENTRYPOINT ["/usr/local/bin/entrypoint"]

USER ${USER_UID}
