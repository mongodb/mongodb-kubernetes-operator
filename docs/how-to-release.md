
## How to Release

* Ensure that [the release notes](./RELEASE_NOTES.md) are up to date for this release.
    * All completed tickets for this release can be seen by running `scripts/dev/open_tickets_for_this_release.py`

* Run the `Create Release PR` GitHub Action
    * In the GitHub UI:
        * `Actions` > `Create Release PR` > `Workflow Dispatch` (run on the master branch)
        
* Review and Approve the release PR that is created by this action.
    * Upon approval, all new images for this release will be built and released, and a Github release draft will be created.

* Review and publish the new GitHub release draft.
