package internal

type Interceptors[T any] []func(T) (T, func())

func (data Interceptors[T]) Apply(t T) (T, func()) {
	cancels := make([]func(), 0, len(data))

	for _, item := range data {
		var cancel func()
		if t, cancel = item(t); cancel != nil {
			cancels = append(cancels, cancel)
		}
	}

	cancel := func() {
		for i := len(cancels) - 1; i >= 0; i-- {
			cancels[i]()
		}
	}

	return t, cancel
}
