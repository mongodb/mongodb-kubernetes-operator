package main

import (
	"fmt"
	"os"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/apis"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/controller"
	"go.uber.org/zap"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
)

func configureLogger() (*zap.Logger, error) {
	// TODO: configure non development logger
	logger, err := zap.NewDevelopment()
	zap.ReplaceGlobals(logger)
	return logger, err
}

func hasRequiredVariables(logger *zap.Logger, envVariables ...string) bool {
	allPresent := true
	for _, envVariable := range envVariables {
		if _, envSpecified := os.LookupEnv(envVariable); !envSpecified {
			logger.Error(fmt.Sprintf("required environment variable %s not found", envVariable))
			allPresent = false
		}
	}
	return allPresent
}

func main() {
	log, err := configureLogger()
	if err != nil {
		os.Exit(1)
	}

	if !hasRequiredVariables(log, "AGENT_IMAGE") {
		os.Exit(1)
	}

	// Get watch namespace from environment variable.
	namespace, nsSpecified := os.LookupEnv("WATCH_NAMESPACE")
	if !nsSpecified {
		os.Exit(1)
	}

	// If namespace is a wildcard use the empty string to represent all namespaces
	watchNamespace := ""
	if namespace == "*" {
		log.Info("Watching all namespaces")
	} else {
		watchNamespace = namespace
		log.Sugar().Infof("Watching namespace: %s", watchNamespace)
	}

	// Get a config to talk to the apiserver
	cfg, err := config.GetConfig()
	if err != nil {
		os.Exit(1)
	}

	// Create a new Cmd to provide shared dependencies and start components
	mgr, err := manager.New(cfg, manager.Options{
		Namespace: watchNamespace,
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
