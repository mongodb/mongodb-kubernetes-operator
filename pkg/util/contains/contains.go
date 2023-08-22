package contains

import (
	"reflect"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/constants"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func String(slice []string, s string) bool {
	for _, elem := range slice {
		if elem == s {
			return true
		}
	}
	return false
}

func Sha256(slice []string) bool {
	return String(slice, constants.Sha256)
}

func Sha1(slice []string) bool {
	return String(slice, constants.Sha1)
}

func X509(slice []string) bool {
	return String(slice, constants.X509)
}

func NamespacedName(nsNames []types.NamespacedName, nsName types.NamespacedName) bool {
	for _, elem := range nsNames {
		if elem == nsName {
			return true
		}
	}
	return false
}

func AccessMode(accessModes []corev1.PersistentVolumeAccessMode, mode corev1.PersistentVolumeAccessMode) bool {
	for _, elem := range accessModes {
		if elem == mode {
			return true
		}
	}
	return false
}

func OwnerReferences(ownerRefs []metav1.OwnerReference, ownerRef metav1.OwnerReference) bool {
	for _, elem := range ownerRefs {
		if reflect.DeepEqual(elem, ownerRef) {
			return true
		}
	}
	return false
}
