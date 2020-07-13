package scram

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSecretHasExpired(t *testing.T) {

	now := func() time.Time {
		return time.Now()
	}
	//time.Now = now

	s := corev1.Secret{}
	s.ObjectMeta.CreationTimestamp = metav1.Time{
		Time: time.Now(),
	}

	assert.False(t, secretHasExpired(s))

	s.ObjectMeta.CreationTimestamp = metav1.Time{
		Time: s.ObjectMeta.CreationTimestamp.Add(time.Hour * 24),
	}

	assert.False(t, secretHasExpired(s))

	s.ObjectMeta.CreationTimestamp = metav1.Time{
		Time: s.ObjectMeta.CreationTimestamp.Add(time.Hour * 24 * 10),
	}

	assert.True(t, secretHasExpired(s))
}
