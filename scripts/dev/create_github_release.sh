#!/usr/bin/env bash

version="$(jq -r '."mongodb-kubernetes-operator"' < ./release.json)"
gh release create v"${version}" --title "MongoDB Kubernetes Operator ${version}" --draft --notes-file ./dev_notes/RELEASE_NOTES.md

# move the release notes
cp ./dev_notes/RELEASE_NOTES.md "./dev_notes/past_release_notes/$v{version}.md"