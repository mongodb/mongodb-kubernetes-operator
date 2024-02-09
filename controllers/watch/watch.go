package watch

import (
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/contains"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// ResourceWatcher implements handler.EventHandler and is used to trigger reconciliation when
// a watched object changes. It's designed to only be used for a single type of object.
// If multiple types should be watched, one ResourceWatcher for each type should be used.
type ResourceWatcher struct {
	watched map[types.NamespacedName][]types.NamespacedName
}

// New will create a new ResourceWatcher with no watched objects.
func New() ResourceWatcher {
	return ResourceWatcher{
		watched: make(map[types.NamespacedName][]types.NamespacedName),
	}
}

// Watch will add a new object to watch.
func (w ResourceWatcher) Watch(watchedName, dependentName types.NamespacedName) {
	existing, hasExisting := w.watched[watchedName]
	if !hasExisting {
		existing = []types.NamespacedName{}
	}

	// Check if resource is already being watched.
	if contains.NamespacedName(existing, dependentName) {
		return
	}

	w.watched[watchedName] = append(existing, dependentName)
}

func (w ResourceWatcher) Create(event event.CreateEvent, queue workqueue.RateLimitingInterface) {
	w.handleEvent(event.Object, queue)
}

func (w ResourceWatcher) Update(event event.UpdateEvent, queue workqueue.RateLimitingInterface) {
	w.handleEvent(event.ObjectOld, queue)
}

func (w ResourceWatcher) Delete(event event.DeleteEvent, queue workqueue.RateLimitingInterface) {
	w.handleEvent(event.Object, queue)
}

func (w ResourceWatcher) Generic(event event.GenericEvent, queue workqueue.RateLimitingInterface) {
	w.handleEvent(event.Object, queue)
}

// handleEvent is called when an event is received for an object.
// It will check if the object is being watched and trigger a reconciliation for
// the dependent object.
func (w ResourceWatcher) handleEvent(meta metav1.Object, queue workqueue.RateLimitingInterface) {
	changedObjectName := types.NamespacedName{
		Name:      meta.GetName(),
		Namespace: meta.GetNamespace(),
	}

	// Enqueue reconciliation for each dependent object.
	for _, reconciledObjectName := range w.watched[changedObjectName] {
		queue.Add(reconcile.Request{
			NamespacedName: reconciledObjectName,
		})
	}
}
