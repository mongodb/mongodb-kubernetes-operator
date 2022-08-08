package securitycontext

import (
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/container"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/podtemplatespec"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/envvar"
)

const (
	ManagedSecurityContextEnv = "MANAGED_SECURITY_CONTEXT"
)

func WithDefaultSecurityContextsModifications() (podtemplatespec.Modification, container.Modification) {
	managedSecurityContext := envvar.ReadBool(ManagedSecurityContextEnv)
	configureContainerSecurityContext := container.NOOP()
	configurePodSpecSecurityContext := podtemplatespec.NOOP()
	if !managedSecurityContext {
		configurePodSpecSecurityContext = podtemplatespec.WithSecurityContext(podtemplatespec.DefaultPodSecurityContext())
		configureContainerSecurityContext = container.WithSecurityContext(container.DefaultSecurityContext())
	}

	return configurePodSpecSecurityContext, configureContainerSecurityContext
}
