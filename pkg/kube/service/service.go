package service

import (
	corev1 "k8s.io/api/core/v1"
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
