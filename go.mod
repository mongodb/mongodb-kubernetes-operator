module github.com/mongodb/mongodb-kubernetes-operator

go 1.14

require (
	github.com/ghodss/yaml v1.0.1-0.20190212211648-25d852aebe32
	github.com/go-logr/logr v0.3.0
	github.com/hashicorp/go-multierror v1.0.0
	github.com/imdario/mergo v0.3.10
	github.com/klauspost/compress v1.9.8 // indirect
	github.com/onsi/ginkgo v1.14.1
	github.com/onsi/gomega v1.10.2
	github.com/pkg/errors v0.9.1
	github.com/stretchr/objx v0.3.0
	github.com/stretchr/testify v1.5.1
	github.com/xdg/stringprep v1.0.0
	go.mongodb.org/mongo-driver v1.3.2
	go.uber.org/zap v1.15.0
	golang.org/x/sys v0.0.0-20200625212154-ddb9806d33ae // indirect
	google.golang.org/protobuf v1.25.0 // indirect
	k8s.io/api v0.19.2
	k8s.io/apiextensions-apiserver v0.19.2
	k8s.io/apimachinery v0.19.2
	k8s.io/client-go v0.19.2
	sigs.k8s.io/controller-runtime v0.7.0
	sigs.k8s.io/yaml v1.2.0
)

replace github.com/docker/docker => github.com/moby/moby v17.12.0-ce-rc1.0.20200309214505-aa6a9891b09c+incompatible // Required by Helm

replace github.com/Azure/go-autorest => github.com/Azure/go-autorest v13.3.2+incompatible // Required by OLM
