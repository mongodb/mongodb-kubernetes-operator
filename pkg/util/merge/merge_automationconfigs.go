package merge

import (
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/automationconfig"
)

// AutomationConfigs merges the values in "override" into the "original" Wrapper.
// Merging is done by name for processes.
func AutomationConfigs(original, override automationconfig.AutomationConfig) automationconfig.AutomationConfig {
	original.Processes = mergeProcesses(original.Processes, override.Processes)
	return original
}

func mergeProcesses(original, override []automationconfig.Process) []automationconfig.Process {
	mergedProcesses := append([]automationconfig.Process{}, original...)
	for _, overrideProcess := range override {
		correspondingIndex := getProcessIndexByName(overrideProcess.Name, original)
		if correspondingIndex == -1 {
			continue
		}
		mergedProcesses[correspondingIndex] = mergeProcess(original[correspondingIndex], overrideProcess)
	}
	return mergedProcesses
}

func getProcessIndexByName(desiredProcessName string, originalProcesses []automationconfig.Process) int {
	for i := range originalProcesses {
		if originalProcesses[i].Name == desiredProcessName {
			return i
		}
	}
	return -1
}

func mergeProcess(original, override automationconfig.Process) automationconfig.Process {
	// TODO: in order to override the disabled field, we just need this one field. We can handle all fields in a future change.
	original.Disabled = override.Disabled
	original.LogRotate = override.LogRotate
	return original
}
