package main

import (
	"fmt"
	"unixcycle/unixcycle"
)

func main() {
	myComp, _ := NewMyTestComponent()
	exitSignal := unixcycle.NewManager().
		Add(
			unixcycle.Make[myTestComponent](func() *myTestComponent {
				return myComp
			})).
		Run()

	fmt.Printf("Exit signal: %q\n", exitSignal)
}

type myTestComponent struct {
	stopwork chan bool
}

func NewMyTestComponent() (*myTestComponent, error) {
	return &myTestComponent{stopwork: make(chan bool)}, nil
}

func (c *myTestComponent) Setup() error {
	fmt.Printf("Setting up myTestComponent\n")
	return nil
}

func (c *myTestComponent) Start() error {
	fmt.Printf("Starting myTestComponent\n")
	<-c.stopwork
	fmt.Printf("received stopwork signal\n")
	// Do some blocking work
	return nil
}

func (c *myTestComponent) Close() error {
	fmt.Printf("Closing myTestComponent\n")
	c.stopwork <- true
	return nil
}
