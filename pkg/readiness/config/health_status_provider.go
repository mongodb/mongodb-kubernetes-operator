package config

import (
	"encoding/json"
	"io/ioutil"
	"os"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/readiness/health"
)

// HealthStatusProvider is an interface for providing the 'health.Status' object to the readiness probe. Abstracting out
// this to the interface allows to perform unit-testing easier as allows to modify the config on-the-fly.
type HealthStatusProvider interface {
	HealthStatus() (health.Status, error)
}

// FileHealthStatusProvider reads the health.Status data from filepath
type FileHealthStatusProvider struct {
	HealthStatusFilePath string
}

func (f FileHealthStatusProvider) HealthStatus() (health.Status, error) {
	var health health.Status
	fd, err := os.Open(f.HealthStatusFilePath)
	if err != nil {
		return health, err
	}
	defer fd.Close()

	data, err := ioutil.ReadAll(fd)
	if err != nil {
		return health, err
	}

	err = json.Unmarshal(data, &health)
	return health, err
}
