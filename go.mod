module github.com/mongodb/mongodb-kubernetes-operator

go 1.14

require (
	github.com/blang/semver v3.5.1+incompatible
	github.com/go-logr/logr v0.4.0
	github.com/hashicorp/go-multierror v1.1.1
	github.com/imdario/mergo v0.3.12
	github.com/klauspost/compress v1.9.8 // indirect
	github.com/pkg/errors v0.9.1
	github.com/spf13/cast v1.3.1
	github.com/stretchr/objx v0.3.0
	github.com/stretchr/testify v1.7.0
	github.com/xdg/stringprep v1.0.0
	go.mongodb.org/mongo-driver v1.5.1
	go.uber.org/zap v1.16.0
	k8s.io/api v0.20.2
	k8s.io/apimachinery v0.21.0
	k8s.io/client-go v0.20.2
	sigs.k8s.io/controller-runtime v0.8.3
	sigs.k8s.io/yaml v1.2.0
)

replace github.com/docker/docker => github.com/moby/moby v17.12.0-ce-rc1.0.20200309214505-aa6a9891b09c+incompatible // Required by Helm

replace github.com/Azure/go-autorest => github.com/Azure/go-autorest v13.3.2+incompatible // Required by OLM
