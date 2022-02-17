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
		if mdb.Spec.Prometheus == nil {
			return
		}

		promConfig := automationconfig.NewDefaultPrometheus(mdb.Spec.Prometheus.Username)
		promConfig.Scheme = "http"
		promConfig.Password = password

		if mdb.Spec.Prometheus.Port > 0 {
			promConfig.ListenAddress = fmt.Sprintf("%s:%d", listenAddress, mdb.Spec.Prometheus.Port)
		}

		if mdb.Spec.Prometheus.MetricsPath != "" {
			promConfig.MetricsPath = mdb.Spec.Prometheus.MetricsPath
		}

		config.Prometheus = &promConfig
	}
}
