module github.com/mongodb/mongodb-kubernetes-operator/mongodb-kubernetes-operator

go 1.13

require (
	github.com/blang/semver v3.5.0+incompatible
	github.com/google/uuid v1.1.1
	github.com/imdario/mergo v0.3.8
	github.com/mongodb/mongodb-kubernetes-operator v0.0.9
	github.com/pkg/errors v0.9.1
	github.com/spf13/cast v1.3.1
	github.com/stretchr/testify v1.4.0
	github.com/xdg/stringprep v1.0.0
	go.uber.org/zap v1.13.0
	k8s.io/api v0.15.9 // kubernetes-1.15.9
	k8s.io/apimachinery v0.15.9 // kubernetes-1.15.9
	k8s.io/client-go v0.15.9 // kubernetes-1.15.9
	k8s.io/code-generator v0.15.9 // kubernetes-1.15.9
	sigs.k8s.io/controller-runtime v0.3.0
)
replace github.com/docker/docker => github.com/moby/moby v0.7.3-0.20190826074503-38ab9da00309 // Required by Helm

replace github.com/openshift/api => github.com/openshift/api v0.0.0-20190924102528-32369d4db2ad // Required until https://github.com/operator-framework/operator-lifecycle-manager/pull/1241 is resolved
