package merge

import (
	"testing"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/automationconfig"
	"github.com/stretchr/testify/assert"
)

func TestMergeAutomationConfigs(t *testing.T) {
	original, err := automationconfig.NewBuilder().
		SetName("test-ac").
		SetMembers(3).
		Build()

	assert.NoError(t, err)
	override, err := automationconfig.NewBuilder().
		SetName("test-ac").
		SetMembers(3).
		AddProcessModification(func(i int, process *automationconfig.Process) {
			// set a single process to be disabled.
			process.Disabled = i == 1
		}).Build()

	assert.NoError(t, err)

	for _, p := range original.Processes {
		assert.False(t, p.Disabled)
	}

	assert.False(t, override.Processes[0].Disabled)
	assert.True(t, override.Processes[1].Disabled)
	assert.False(t, override.Processes[2].Disabled)

	mergedAc := AutomationConfigs(original, override)
	assert.False(t, mergedAc.Processes[0].Disabled)
	assert.True(t, mergedAc.Processes[1].Disabled)
	assert.False(t, mergedAc.Processes[2].Disabled)
}

func TestMergeAutomationConfigs_NonExistentMember(t *testing.T) {
	original, err := automationconfig.NewBuilder().
		SetName("test-ac").
		SetMembers(3).
		Build()

	assert.NoError(t, err)
	override, err := automationconfig.NewBuilder().
		SetName("test-ac-0").
		SetMembers(3).
		AddProcessModification(func(i int, process *automationconfig.Process) {
			process.Disabled = i == 1
		}).Build()

	assert.NoError(t, err)

	assert.False(t, override.Processes[0].Disabled)
	assert.True(t, override.Processes[1].Disabled)
	assert.False(t, override.Processes[2].Disabled)

	mergedAc := AutomationConfigs(original, override)

	assert.False(t, mergedAc.Processes[0].Disabled)
	assert.False(t, mergedAc.Processes[1].Disabled, "should not be updated as the name does not match.")
	assert.False(t, mergedAc.Processes[2].Disabled)
}
