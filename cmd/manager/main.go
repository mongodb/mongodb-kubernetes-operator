package main

import (
	"fmt"
	"os"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/api/v1"
	"github.com/mongodb/mongodb-kubernetes-operator/controllers"
	"github.com/mongodb/mongodb-kubernetes-operator/controllers/construct"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
)

var (
	scheme = runtime.NewScheme()
)

const (
	WatchNamespaceEnv = "WATCH_NAMESPACE"
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
		log.Sugar().Fatalf("Failed to configure logger: %v", err)
	}

	if !hasRequiredVariables(log, construct.AgentImageEnv, construct.VersionUpgradeHookImageEnv, construct.ReadinessProbeImageEnv) {
		os.Exit(1)
	}

	// Get watch namespace from environment variable.
	namespace, nsSpecified := os.LookupEnv(WatchNamespaceEnv)
	if !nsSpecified {
		log.Sugar().Fatal("No namespace specified to watch")
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
		log.Sugar().Fatalf("Unable to get config: %v", err)
	}

	// Create a new Cmd to provide shared dependencies and start components
	mgr, err := manager.New(cfg, manager.Options{
		Namespace: watchNamespace,
	})
	if err != nil {
		log.Sugar().Fatalf("Unable to create manager: %v", err)
	}

	log.Info("Registering Components.")

	// Setup Scheme for all resources
	if err := mdbv1.AddToScheme(mgr.GetScheme()); err != nil {
		log.Sugar().Fatalf("Unable to add mdbv1 to scheme: %v", err)
	}

	// Setup Controller.
	if err = controllers.NewReconciler(mgr).SetupWithManager(mgr); err != nil {
		log.Sugar().Fatalf("Unable to create controller: %v", err)
	}
	// +kubebuilder:scaffold:builder

	log.Info("Starting the Cmd.")

	// Start the Cmd
	if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
		log.Sugar().Fatalf("Unable to start manager: %v", err)
	}
}
