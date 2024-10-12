package main

import (
	"fmt"
	"unixcycle/unixcycle"
)

func main() {
	myFactoryFunc := func() (*myTestComponent, error) {
		return &myTestComponent{}, nil
	}
	manager := unixcycle.NewManager(
		unixcycle.Stopper[myTestComponent](myFactoryFunc),
	)

	manager.Run()
}

type myTestComponent struct {
}

func NewMyTestComponent() (*myTestComponent, error) {
	return &myTestComponent{}, nil
}

func (c *myTestComponent) Start() error {
	fmt.Printf("Starting myTestComponent\n")
	return nil
}

func (c *myTestComponent) Stop() error {
	fmt.Printf("Stopping myTestComponent\n")
	return nil
}
