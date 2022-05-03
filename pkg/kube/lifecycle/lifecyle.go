package lifecycle

import corev1 "k8s.io/api/core/v1"

type Modification func(lifecycle *corev1.Lifecycle)

// Apply returns a function which applies a series of Modification functions to a *corev1.Lifecycle
func Apply(modifications ...Modification) Modification {
	return func(lifecycle *corev1.Lifecycle) {
		for _, mod := range modifications {
			mod(lifecycle)
		}
	}
}

// WithPrestopCommand sets the LifeCycles PreStop Exec Command
func WithPrestopCommand(preStopCmd []string) Modification {
	return func(lc *corev1.Lifecycle) {
		if lc.PreStop == nil {
			lc.PreStop = &corev1.LifecycleHandler{}
		}
		if lc.PreStop.Exec == nil {
			lc.PreStop.Exec = &corev1.ExecAction{}
		}
		lc.PreStop.Exec.Command = preStopCmd
	}
}
