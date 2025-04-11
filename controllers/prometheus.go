package controllers

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/api/v1"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/automationconfig"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/secret"

	"k8s.io/apimachinery/pkg/types"
)

const (
	// Keep in sync with api/v1/mongodbcommunity_types.go
	DefaultPrometheusPort = 9216
	ListenAddress         = "0.0.0.0"
)

// PrometheusModification adds Prometheus configuration to AutomationConfig.
func getPrometheusModification(ctx context.Context, getUpdateCreator secret.GetUpdateCreator, mdb mdbv1.MongoDBCommunity) (automationconfig.Modification, error) {
	if mdb.Spec.Prometheus == nil {
		return automationconfig.NOOP(), nil
	}

	secretNamespacedName := types.NamespacedName{Name: mdb.Spec.Prometheus.PasswordSecretRef.Name, Namespace: mdb.Namespace}
	password, err := secret.ReadKey(ctx, getUpdateCreator, mdb.Spec.Prometheus.GetPasswordKey(), secretNamespacedName)
	if err != nil {
		return automationconfig.NOOP(), fmt.Errorf("could not configure Prometheus modification: %s", err)
	}

	var certKey string
	var tlsPEMPath string
	var scheme string

	if mdb.Spec.Prometheus.TLSSecretRef.Name != "" {
		certKey, err = getPemOrConcatenatedCrtAndKey(ctx, getUpdateCreator, mdb.PrometheusTLSSecretNamespacedName())
		if err != nil {
			return automationconfig.NOOP(), err
		}
		tlsPEMPath = tlsPrometheusSecretMountPath + tlsOperatorSecretFileName(certKey)
		scheme = "https"
	} else {
		scheme = "http"
	}

	return func(config *automationconfig.AutomationConfig) {
		promConfig := automationconfig.NewDefaultPrometheus(mdb.Spec.Prometheus.Username)

		promConfig.TLSPemPath = tlsPEMPath
		promConfig.Scheme = scheme
		promConfig.Password = password

		if mdb.Spec.Prometheus.Port > 0 {
			promConfig.ListenAddress = fmt.Sprintf("%s:%d", ListenAddress, mdb.Spec.Prometheus.Port)
		}

		if mdb.Spec.Prometheus.MetricsPath != "" {
			promConfig.MetricsPath = mdb.Spec.Prometheus.MetricsPath
		}

		config.Prometheus = &promConfig
	}, nil
}

// prometheusPort returns a `corev1.ServicePort` to be configured in the StatefulSet
// for the Prometheus endpoint. This function will only return a new Port when
// Prometheus has been configured, and nil otherwise.
func prometheusPort(mdb mdbv1.MongoDBCommunity) *corev1.ServicePort {
	if mdb.Spec.Prometheus != nil {
		return &corev1.ServicePort{
			Port: int32(mdb.Spec.Prometheus.GetPort()),
			Name: "prometheus",
		}
	}
	return nil
}
