package unixcycle

type Stoppable interface {
	Stop() error
}

type Startable interface {
	Start() error
}

type StartStopper interface {
	Stoppable
	Startable
}

var _ StartStopper = &component{}

type component struct {
	StartFunc Startable
	StopFunc  Stoppable
}

func (c *component) Start() error {
	return c.StartFunc.Start()
}

func (c *component) Stop() error {
	return c.StopFunc.Stop()
}
