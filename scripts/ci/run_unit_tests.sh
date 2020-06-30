#!/bin/sh

. venv/bin/activate
pip3 install -r ./requirements.txt
python scripts/dev/dockerfile_generator.py "unittest" > Dockerfile
docker build . -f Dockerfile -t "unit-tests:${version_id:?}"
docker run "unit-tests:${version_id:?}"
