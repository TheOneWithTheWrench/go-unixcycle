package unixcycle

type make[T any] *T
type makeFunc[T any] func() *T
type makeErrFunc[T any] func() (*T, error)

type StarterConstraint[T any] interface {
	*T
	Startable
}

type StopperConstraint[T any] interface {
	*T
	Stoppable
}

type StarterStopperConstraint[T any] interface {
	*T
	StartStopper
}

func Starter[T any, CI StarterConstraint[T]](x any) *component {
	return wrap[T](x)
}

func Stopper[T any, CI StopperConstraint[T]](x any) *component {
	return wrap[T](x)
}

// Make function is a convenience function to create a component from a function that returns a pointer to a struct that implements unixcycle.StartStopper
func Make[T any, CI StarterStopperConstraint[T]](x any) *component {
	return wrap[T](x)
}

func wrap[T any](x any) *component {
	var (
		obj       *T
		err       error
		component = &component{}
	)

	switch x := x.(type) {
	case make[T]:
		obj = x
	case makeFunc[T]:
		obj = x()
	case makeErrFunc[T]:
		obj, err = x()
	default:
		panic("invalid type")
	}
	if err != nil {
		panic(err)
	}

	var something interface{} = obj
	if casted, ok := something.(Startable); ok {
		component.StartFunc = casted
	}
	if casted, ok := something.(Stoppable); ok {
		component.StopFunc = casted
	}

	return component
}
