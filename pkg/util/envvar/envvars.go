package envvar

import (
	"sort"

	corev1 "k8s.io/api/core/v1"
)

func MergeWithOverride(existing, desired []corev1.EnvVar) []corev1.EnvVar {
	envMap := make(map[string]corev1.EnvVar)
	for _, env := range existing {
		envMap[env.Name] = env
	}

	for _, env := range desired {
		envMap[env.Name] = env
	}

	var mergedEnv []corev1.EnvVar
	for _, env := range envMap {
		mergedEnv = append(mergedEnv, env)
	}

	sort.SliceStable(mergedEnv, func(i, j int) bool {
		return mergedEnv[i].Name < mergedEnv[j].Name
	})
	return mergedEnv
}
