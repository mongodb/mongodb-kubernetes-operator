#!/usr/bin/env bash

pip3 install black
black -q --check scripts test pkg scripts
