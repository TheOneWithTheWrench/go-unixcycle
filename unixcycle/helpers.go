package unixcycle

import "fmt"

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
	var (
		component = &component{}
		obj       = wrap[T](x)
	)

	var untyped interface{} = obj
	if casted, ok := untyped.(Startable); ok {
		component.StartFunc = casted
	} else {
		panic(fmt.Sprintf("unsupported type %T", x))
	}

	return component
}

func Stopper[T any, CI StopperConstraint[T]](x any) *component {
	var (
		component = &component{}
		obj       = wrap[T](x)
	)

	var untyped interface{} = obj
	if casted, ok := untyped.(Stoppable); ok {
		component.StopFunc = casted
	} else {
		panic(fmt.Sprintf("unsupported type %T", x))
	}

	return component
}

// Make function is a convenience function to create a component from a function that returns a pointer to a struct that implements unixcycle.StartStopper
func Make[T any, CI StarterStopperConstraint[T]](x any) *component {
	var (
		component = &component{}
		obj       = wrap[T](x)
	)

	var untyped interface{} = obj
	if casted, ok := untyped.(StartStopper); ok {
		component.StartFunc = casted
		component.StopFunc = casted
	} else {
		panic(fmt.Sprintf("unsupported type %T", x))
	}

	return component
}

func wrap[T any](x any) *T {
	var (
		obj *T
		err error
	)

	switch x := x.(type) {
	case *T:
		obj = x
	case func() *T:
		obj = x()
	case func() (*T, error):
		obj, err = x()
	default:
		panic(fmt.Sprintf("unsupported type %T", x))
	}
	if err != nil {
		panic(err)
	}

	return obj
}
