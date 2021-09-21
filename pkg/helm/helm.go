package helm

import (
	"fmt"
	"os/exec"
	"strings"
)

// Uninstall uninstalls a helm chart of the given name. There is no error in the case
// of the helm chart not existing.
func Uninstall(chartName string) error {
	helmArgs := []string{"uninstall", chartName}
	return executeHelmCommand(helmArgs, isNotFoundMessage)
}

// Install a helm chert at the given path with the given name and the provided set arguments.
func Install(chartPath, chartName string, args map[string]string) error {
	helmArgs := []string{"install", "--skip-crds"}
	helmArgs = append(helmArgs, mapToHelmArgs(args)...)
	helmArgs = append(helmArgs, chartName, chartPath)
	return executeHelmCommand(helmArgs, nil)
}

func isNotFoundMessage(s string) bool {
	return strings.Contains(s, "not found")
}

// executeHelmCommand accepts a list of arguments that should be passed to the helm command
// and a predicate that when returning true, indicates that the error message should be ignored.
func executeHelmCommand(args []string, messagePredicate func(string) bool) error {
	cmd := exec.Command("helm", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if messagePredicate != nil && messagePredicate(string(output)) {
			return nil
		}
		return fmt.Errorf("error executing command: %s %s", err, output)
	}
	return nil
}

// mapToHelmArgs accepts a map of string to string and returns a list of arguments
// that can be passed to a shell helm command.
func mapToHelmArgs(m map[string]string) []string {
	var args []string
	for k, v := range m {
		args = append(args, "--set", fmt.Sprintf("%s=%s", k, v))
	}
	return args
}
