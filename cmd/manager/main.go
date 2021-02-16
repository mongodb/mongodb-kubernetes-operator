package main

import (
	"fmt"
	"os"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/api/v1"
	controllers "github.com/mongodb/mongodb-kubernetes-operator/controllers"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	manager "sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(mdbv1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

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
		setupLog.Error(err, "Unable to get config")
		os.Exit(1)
	}

	// Create a new Cmd to provide shared dependencies and start components
	mgr, err := manager.New(cfg, manager.Options{
		Namespace: watchNamespace,
	})
	if err != nil {
		setupLog.Error(err, "Unable to create manager")
		os.Exit(1)
	}

	log.Info("Registering Components.")

	// Setup Scheme for all resources
	if err := mdbv1.AddToScheme(mgr.GetScheme()); err != nil {
		setupLog.Error(err, "Unable to add mdbv1 to scheme")
		os.Exit(1)
	}

	// Setup Controller.
	if err = controllers.NewReconciler(mgr, nil).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "Unable to create controller")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	log.Info("Starting the Cmd.")

	// Start the Cmd
	if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "Unable to start manager")
		os.Exit(1)
	}
}
