package watch

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/types"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/api/v1"

	"github.com/stretchr/testify/assert"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllertest"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/workqueue"

	"sigs.k8s.io/controller-runtime/pkg/event"
)

func TestWatcher(t *testing.T) {
	ctx := context.Background()
	obj := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod",
			Namespace: "namespace",
		},
	}
	objNsName := types.NamespacedName{Name: obj.Name, Namespace: obj.Namespace}

	mdb1 := mdbv1.MongoDBCommunity{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mdb1",
			Namespace: "namespace",
		},
	}

	mdb2 := mdbv1.MongoDBCommunity{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mdb2",
			Namespace: "namespace",
		},
	}

	t.Run("Non-watched object", func(t *testing.T) {
		watcher := New()
		queue := controllertest.Queue{Interface: workqueue.New()}

		watcher.Create(ctx, event.CreateEvent{
			Object: obj,
		}, &queue)

		// Ensure no reconciliation is queued if object is not watched.
		assert.Equal(t, 0, queue.Len())
	})

	t.Run("Multiple objects to reconcile", func(t *testing.T) {
		watcher := New()
		queue := controllertest.Queue{Interface: workqueue.New()}
		watcher.Watch(ctx, objNsName, mdb1.NamespacedName())
		watcher.Watch(ctx, objNsName, mdb2.NamespacedName())

		watcher.Create(ctx, event.CreateEvent{
			Object: obj,
		}, &queue)

		// Ensure multiple reconciliations are enqueued.
		assert.Equal(t, 2, queue.Len())
	})

	t.Run("Create event", func(t *testing.T) {
		watcher := New()
		queue := controllertest.Queue{Interface: workqueue.New()}
		watcher.Watch(ctx, objNsName, mdb1.NamespacedName())

		watcher.Create(ctx, event.CreateEvent{
			Object: obj,
		}, &queue)

		assert.Equal(t, 1, queue.Len())
	})

	t.Run("Update event", func(t *testing.T) {
		watcher := New()
		queue := controllertest.Queue{Interface: workqueue.New()}
		watcher.Watch(ctx, objNsName, mdb1.NamespacedName())

		watcher.Update(ctx, event.UpdateEvent{
			ObjectOld: obj,
			ObjectNew: obj,
		}, &queue)

		assert.Equal(t, 1, queue.Len())
	})

	t.Run("Delete event", func(t *testing.T) {
		watcher := New()
		queue := controllertest.Queue{Interface: workqueue.New()}
		watcher.Watch(ctx, objNsName, mdb1.NamespacedName())

		watcher.Delete(ctx, event.DeleteEvent{
			Object: obj,
		}, &queue)

		assert.Equal(t, 1, queue.Len())
	})

	t.Run("Generic event", func(t *testing.T) {
		watcher := New()
		queue := controllertest.Queue{Interface: workqueue.New()}
		watcher.Watch(ctx, objNsName, mdb1.NamespacedName())

		watcher.Generic(ctx, event.GenericEvent{
			Object: obj,
		}, &queue)

		assert.Equal(t, 1, queue.Len())
	})
}

func TestWatcherAdd(t *testing.T) {
	ctx := context.Background()
	watcher := New()
	assert.Empty(t, watcher.watched)

	watchedName := types.NamespacedName{Name: "object", Namespace: "namespace"}

	mdb1 := mdbv1.MongoDBCommunity{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mdb1",
			Namespace: "namespace",
		},
	}
	mdb2 := mdbv1.MongoDBCommunity{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mdb2",
			Namespace: "namespace",
		},
	}

	// Ensure single object can be added to empty watchlist.
	watcher.Watch(ctx, watchedName, mdb1.NamespacedName())
	assert.Len(t, watcher.watched, 1)
	assert.Equal(t, []types.NamespacedName{mdb1.NamespacedName()}, watcher.watched[watchedName])

	// Ensure object can only be watched once.
	watcher.Watch(ctx, watchedName, mdb1.NamespacedName())
	assert.Len(t, watcher.watched, 1)
	assert.Equal(t, []types.NamespacedName{mdb1.NamespacedName()}, watcher.watched[watchedName])

	// Ensure a single object can be watched for multiple reconciliations.
	watcher.Watch(ctx, watchedName, mdb2.NamespacedName())
	assert.Len(t, watcher.watched, 1)
	assert.Equal(t, []types.NamespacedName{
		mdb1.NamespacedName(),
		mdb2.NamespacedName(),
	}, watcher.watched[watchedName])
}
