package functions

// RunSequentially executes a series of functions sequentially. Each function returns a boolean
// indicating if the function was successful, and an error indicating if something went wrong.
// if any function returns an error, an early exit happens. The first parameter indicates if the functions
// should run in the order provided. A value of false indicates they should run in reverse.
func RunSequentially(runSequentially bool, funcs ...func() (bool, error)) (bool, error) {
	if runSequentially {
		return runInOrder(funcs...)
	}
	return runReversed(funcs...)
}

func runInOrder(funcs ...func() (bool, error)) (bool, error) {
	for _, fn := range funcs {
		successful, err := fn()
		if err != nil {
			return successful, err
		}
		if !successful {
			return false, nil
		}
	}
	return true, nil
}

func runReversed(funcs ...func() (bool, error)) (bool, error) {
	for i := len(funcs) - 1; i >= 0; i-- {
		successful, err := funcs[i]()
		if err != nil {
			return successful, err
		}
		if !successful {
			return false, nil
		}
	}
	return true, nil
}
