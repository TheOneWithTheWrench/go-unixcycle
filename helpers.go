package unixcycle

import (
	"fmt"
)

// Make function is a convenience function to create a component from a function that returns a pointer to a struct that implements unixcycle.StartStopper
func Make[T any, SSC starterConstraint[T], MC makerConstraint[T]](x MC) Component {
	var (
		obj = wrap[T](x)
	)

	var untyped any = obj
	return untyped.(Component)
}

func Setup(setupFunc func() error) *setupComponent {
	return &setupComponent{setupFunc: setupFunc}
}

func Starter(startFunc func() error) *starterComponent {
	return &starterComponent{startFunc: startFunc}
}

func Closer(closeFunc func() error) *closerComponent {
	return &closerComponent{closeFunc: closeFunc}
}

// Type constraint to allow either makeFunc[T] or makeErrorFunc[T]
type makerConstraint[T any] interface {
	*T | ~func() *T | func() (*T, error)
}

type starterConstraint[T any] interface {
	*T
	startable
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
