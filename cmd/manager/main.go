package main

import (
	"fmt"
	"go.uber.org/zap"
	"os"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/apis"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/controller"

	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
)

// Change below variables to serve metrics on different host or port.
var (
	metricsHost               = "0.0.0.0"
	metricsPort         int32 = 8383
	operatorMetricsPort int32 = 8686
)

func main() {
	// TODO: configure non development logger
	log, err := zap.NewDevelopment()
	if err != nil {
		os.Exit(1)
	}
	// get watch namespace from environment variable
	namespace, nsSpecified := os.LookupEnv("WATCH_NAMESPACE")
	if !nsSpecified {
		os.Exit(1)
	}

	log.Info(fmt.Sprintf("Watching namespace: %s", namespace))

	// Get a config to talk to the apiserver
	cfg, err := config.GetConfig()
	if err != nil {
		os.Exit(1)
	}

	// Create a new Cmd to provide shared dependencies and start components
	mgr, err := manager.New(cfg, manager.Options{
		Namespace:          "temp",
		MetricsBindAddress: fmt.Sprintf("%s:%d", metricsHost, metricsPort),
	})
	if err != nil {
		os.Exit(1)
	}

	log.Info("Registering Components.")

	// Setup Scheme for all resources
	if err := apis.AddToScheme(mgr.GetScheme()); err != nil {
		os.Exit(1)
	}

	// Setup all Controllers
	if err := controller.AddToManager(mgr); err != nil {
		os.Exit(1)
	}

	log.Info("Starting the Cmd.")

	// Start the Cmd
	if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
		os.Exit(1)
	}
}
