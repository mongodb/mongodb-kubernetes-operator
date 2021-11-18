
## How to Release

* Prepare release PR:
    * Update any changing versions in [release.json](../release.json).
    * Ensure that [the release notes](./RELEASE_NOTES.md) are up to date for this release.
    * Run `python scripts/ci/update_release.py` to update the relevant yaml manifests.
    * Copy `CRD`s to Helm Chart
      - `cp config/crd/bases/mongodbcommunity.mongodb.com_mongodbcommunity.yaml helm-charts/charts/community-operator-crds/templates/mongodbcommunity.mongodb.com_mongodbcommunity.yaml`
    * Commit all changes.
    * Create a PR with the title `Release MongoDB Kubernetes Operator v<operator-version>` (the title must match this pattern)
    
* Have this PR Reviewed and Approved.
    * Upon approval, all new images for this release will be built and released, and a Github release draft will be created.
    * Once tests have passed, merge the release PR.

* Review and publish the new GitHub release draft.
