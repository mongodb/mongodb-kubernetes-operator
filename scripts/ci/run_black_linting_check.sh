#!/usr/bin/env bash

pip3 install black
if ! black -q --check scripts test pkg scripts
then
    echo "Black linting check failed!"
    exit 1
fi

exit 0
