package monitoring

import (
	"fmt"
	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/api/v1"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/automationconfig"
)

const (
	listenAddress = "0.0.0.0"
)

// PrometheusModification adds Prometheus configuration to the AutomationConfig.
func PrometheusModification(mdb mdbv1.MongoDBCommunity, password string) automationconfig.Modification {
	return func(config *automationconfig.AutomationConfig) {
		if mdb.Spec.Metrics == nil {
			return
		}

		promConfig := automationconfig.NewDefaultPrometheus(mdb.Spec.Metrics.Prometheus.Username)
		promConfig.Scheme = "http"
		promConfig.Password = password

		if mdb.Spec.Metrics.Prometheus.Port > 0 {
			promConfig.ListenAddress = fmt.Sprintf("%s:%d", listenAddress, mdb.Spec.Metrics.Prometheus.Port)
		}

		if mdb.Spec.Metrics.Prometheus.MetricsPath != "" {
			promConfig.MetricsPath = mdb.Spec.Metrics.Prometheus.MetricsPath
		}

		config.Prometheus = &promConfig
	}
}
