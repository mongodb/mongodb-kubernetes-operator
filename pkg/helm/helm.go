package helm

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// Uninstall uninstalls a helm chart of the given name. There is no error in the case
// of the helm chart not existing.
func Uninstall(chartName string) error {
	helmArgs := []string{"uninstall", chartName}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := exec.Command("helm", helmArgs...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	if err != nil {
		if strings.Contains(stderr.String(), "not found") {
			fmt.Printf("%s was not found, nothing to uninstall.\n", chartName)
			return nil
		}
		fmt.Println(stderr.String())
		fmt.Println(stdout.String())
	}

	fmt.Printf("uninstalling chart %s.\n", chartName)
	return nil
}

// Install a helm chert at the given path with the given name and the provided set arguments.
func Install(chartPath, chartName string, args map[string]string) error {
	helmArgs := []string{"install"}
	helmArgs = append(helmArgs, mapToHelmArgs(args)...)
	helmArgs = append(helmArgs, chartName, chartPath)

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := exec.Command("helm", helmArgs...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	if err != nil {
		fmt.Println(stderr.String())
		fmt.Println(stdout.String())
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
