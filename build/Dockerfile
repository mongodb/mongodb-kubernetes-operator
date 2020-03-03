FROM ubuntu:16.04

ENV OPERATOR=/usr/local/bin/mongodb-kubernetes-operator \
    USER_UID=1001 \
    USER_NAME=mongodb-kubernetes-operator

# install operator binary
COPY build/_output/bin/mongodb-kubernetes-operator ${OPERATOR}

COPY build/bin /usr/local/bin
RUN  /usr/local/bin/user_setup

ENTRYPOINT ["/usr/local/bin/entrypoint"]

USER ${USER_UID}
