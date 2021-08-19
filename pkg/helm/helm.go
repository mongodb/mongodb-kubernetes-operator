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
	cmd := exec.Command("helm", helmArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if strings.Contains(string(output), "not found") {
			return nil
		}
		return fmt.Errorf("error executing command: %s %s", err, output)
	}
	return nil
}

// Install a helm chert at the given path with the given name and the provided set arguments.
func Install(chartPath, chartName string, args map[string]string) error {
	helmArgs := []string{"install"}
	helmArgs = append(helmArgs, mapToHelmArgs(args)...)
	helmArgs = append(helmArgs, chartName, chartPath)
	cmd := exec.Command("helm", helmArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error executing command: %s %s", err, output)
	}
	return err
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
