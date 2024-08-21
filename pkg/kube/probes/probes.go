package probes

import corev1 "k8s.io/api/core/v1"

type Modification func(*corev1.Probe)

func Apply(funcs ...Modification) Modification {
	return func(probe *corev1.Probe) {
		for _, f := range funcs {
			f(probe)
		}
	}
}

func New(funcs ...Modification) corev1.Probe {
	probe := corev1.Probe{}
	for _, f := range funcs {
		f(&probe)
	}
	return probe
}

func WithExecCommand(cmd []string) Modification {
	return func(probe *corev1.Probe) {
		if probe.ProbeHandler.Exec == nil {
			probe.ProbeHandler.Exec = &corev1.ExecAction{}
		}
		probe.ProbeHandler.Exec.Command = cmd
	}
}

func WithFailureThreshold(failureThreshold int) Modification {
	return func(probe *corev1.Probe) {
		probe.FailureThreshold = int32(failureThreshold) // #nosec G115
	}
}

func WithInitialDelaySeconds(initialDelaySeconds int) Modification {
	return func(probe *corev1.Probe) {
		probe.InitialDelaySeconds = int32(initialDelaySeconds) // #nosec G115
	}
}
func WithSuccessThreshold(successThreshold int) Modification {
	return func(probe *corev1.Probe) {
		probe.SuccessThreshold = int32(successThreshold) // #nosec G115
	}
}
func WithPeriodSeconds(periodSeconds int) Modification {
	return func(probe *corev1.Probe) {
		probe.PeriodSeconds = int32(periodSeconds) // #nosec G115
	}
}
func WithTimeoutSeconds(timeoutSeconds int) Modification {
	return func(probe *corev1.Probe) {
		probe.TimeoutSeconds = int32(timeoutSeconds) // #nosec G115
	}
}

func WithHandler(handler corev1.ProbeHandler) Modification {
	return func(probe *corev1.Probe) {
		probe.ProbeHandler = handler
	}
}
