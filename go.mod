module github.com/mongodb/mongodb-kubernetes-operator

go 1.13

require (
	github.com/blang/semver v3.5.1+incompatible
	github.com/spf13/cast v1.3.1
	github.com/stretchr/testify v1.4.0
	go.uber.org/zap v1.13.0
	k8s.io/api v0.15.9
	k8s.io/apimachinery v0.15.9
	k8s.io/client-go v12.0.0+incompatible // indirect
	sigs.k8s.io/controller-runtime v0.3.0
)
