#!/usr/bin/env bash

pip3 install black
if ! black -q --check ${1}
then
    echo "Black formatting check failed!"
    exit 1
fi

exit 0
