package connectivity

import "time"

func New(modifications ...Modification) Options {
	defaultConnectivity := Options{
		IntervalTime:   1 * time.Second,
		TimeoutTime:    30 * time.Second,
		ContextTimeout: 10 * time.Minute,
	}
	for _, mod := range modifications {
		mod(&defaultConnectivity)
	}
	return defaultConnectivity
}

type Options struct {
	Retries        int
	IntervalTime   time.Duration
	TimeoutTime    time.Duration
	ContextTimeout time.Duration
}

type Modification func(options *Options)

func IntervalTime(intervalTime time.Duration) Modification {
	return func(connectivityOptions *Options) {
		connectivityOptions.IntervalTime = intervalTime
	}
}

func TimeoutTime(timeoutTime time.Duration) Modification {
	return func(connectivityOptions *Options) {
		connectivityOptions.TimeoutTime = timeoutTime
	}
}

func ContextTimeout(contextTimeout time.Duration) Modification {
	return func(connectivityOptions *Options) {
		connectivityOptions.ContextTimeout = contextTimeout
	}
}
