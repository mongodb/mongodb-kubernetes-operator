
#### Config Options

1. `namespace` is the namespace that will be used by scripts/tooling. All of the resources will be deployed here.
2. `operator_name` will be used as the name of the operator deployment, and the name of the operator image when build.
3. `image_type` this can be either `ubi` or `ubuntu` and determines the distro of the images built. (currently only the agent image has multiple distros)
4. `repo_url` the repository that should be used to push/pull all images.
5. `e2e_image` the name of e2e test image that will be built.
6. `version_upgrade_hook_image` the name of the version upgrade post start hook image.
7. `agent_image_ubuntu` the name of the ubuntu agent image.
8. `agent_image_ubi` the name of the ubi agent image.
9. `s3_bucket` the S3 bucket that Dockerfiles will be pushed to as part of the release process. Note: this is only required when running the release tasks locally.
