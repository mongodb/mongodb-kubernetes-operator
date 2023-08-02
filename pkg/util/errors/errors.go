package errors

import (
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func NotFoundError() error {
	return &errors.StatusError{ErrStatus: v1.Status{Reason: v1.StatusReasonNotFound}}
}
