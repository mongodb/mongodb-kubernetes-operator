package service

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Getter interface {
	GetService(objectKey client.ObjectKey) (corev1.Service, error)
}

type Updater interface {
	UpdateService(service corev1.Service) error
}

type Creator interface {
	CreateService(service corev1.Service) error
}

type Deleter interface {
	DeleteService(objectKey client.ObjectKey) error
}

type GetDeleter interface {
	Getter
	Deleter
}

type GetUpdater interface {
	Getter
	Updater
}

type GetUpdateCreator interface {
	Getter
	Updater
	Creator
}

type GetUpdateCreateDeleter interface {
	Getter
	Updater
	Creator
	Deleter
}

func DeleteServiceIfItExists(getterDeleter GetDeleter, serviceName types.NamespacedName) error {
	_, err := getterDeleter.GetService(serviceName)
	if err != nil {
		// If it is not found return
		if apiErrors.IsNotFound(err) {
			return nil
		}
		// Otherwise we got an error when trying to get it
		return fmt.Errorf("can't get service %s: %s", serviceName, err)
	}
	return getterDeleter.DeleteService(serviceName)
}

// Merge merges `source` into `dest`. Both arguments will remain unchanged
// a new service will be created and returned.
// The "merging" process is arbitrary and it only handle specific attributes
func Merge(dest corev1.Service, source corev1.Service) corev1.Service {
	for k, v := range source.ObjectMeta.Annotations {
		dest.ObjectMeta.Annotations[k] = v
	}

	for k, v := range source.ObjectMeta.Labels {
		dest.ObjectMeta.Labels[k] = v
	}

	var cachedNodePorts []int32
	for _, nodePort := range dest.Spec.Ports {
		cachedNodePorts = append(cachedNodePorts, nodePort.NodePort)
	}

	if len(source.Spec.Ports) > 0 {
		portCopy := make([]corev1.ServicePort, len(source.Spec.Ports))
		copy(portCopy, source.Spec.Ports)
		dest.Spec.Ports = portCopy

		for i, cachedNodePort := range cachedNodePorts {
			// Source might not specify NodePort and we shouldn't override existing NodePort value
			if source.Spec.Ports[i].NodePort == 0 {
				dest.Spec.Ports[i].NodePort = cachedNodePort
			}
		}
	}

	dest.Spec.Type = source.Spec.Type
	dest.Spec.LoadBalancerIP = source.Spec.LoadBalancerIP
	dest.Spec.ExternalTrafficPolicy = source.Spec.ExternalTrafficPolicy
	return dest
}

// CreateOrUpdateService will create or update a service in Kubernetes.
func CreateOrUpdateService(getUpdateCreator GetUpdateCreator, desiredService corev1.Service) error {
	namespacedName := types.NamespacedName{Namespace: desiredService.ObjectMeta.Namespace, Name: desiredService.ObjectMeta.Name}
	existingService, err := getUpdateCreator.GetService(namespacedName)

	if err != nil {
		if apiErrors.IsNotFound(err) {
			err = getUpdateCreator.CreateService(desiredService)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	} else {
		mergedService := Merge(existingService, desiredService)
		err = getUpdateCreator.UpdateService(mergedService)
		if err != nil {
			return err
		}
	}
	return nil
}
