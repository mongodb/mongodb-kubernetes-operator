
## How to Release

* Update any finished tickets in [kube-community-next](https://jira.mongodb.org/issues?jql=project%20%3D%20CLOUDP%20AND%20component%20%3D%20%22Kubernetes%20Community%22%20%20AND%20status%20in%20(Resolved%2C%20Closed)%20and%20fixVersion%3D%20kube-community-next%20%20ORDER%20BY%20resolved) to have the version of the release you're doing (kube-community-x.y)

* Create release PR
  1. Increment any image version changes.
  2. Create a github draft release `./scripts/dev/create_github_release.sh`
  3. Reconfigure the Evergreen run to add the release task


* Unblock release task once everything is green

Once the images are released, merge release PR & publish github release