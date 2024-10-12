package main

import (
	"fmt"
	"unixcycle/unixcycle"
)

func main() {
	// myComp, _ := NewMyTestComponent()
	manager := unixcycle.NewManager(
		unixcycle.Stopper[myTestComponent](NewMyTestComponent),
	)

	manager.Run()
}

type myTestComponent struct {
}

func NewMyTestComponent() (*myTestComponent, error) {
	return &myTestComponent{}, nil
}

func (c *myTestComponent) StartButDiff() error {
	fmt.Printf("Starting myTestComponent\n")
	return nil
}

func (c *myTestComponent) Stop() error {
	fmt.Printf("Stopping myTestComponent\n")
	return nil
}
