package unixcycle

type setupable interface {
	Setup() error
}

type startable interface {
	Start() error
}

type closable interface {
	Close() error
}

type Component interface {
	// Start is the long running part of a "Component"
	Start() error
}

var _ Component = &setupComponent{}

type setupComponent struct {
	setupFunc func() error
}

func (s *setupComponent) Setup() error {
	return s.setupFunc()
}

func (s *setupComponent) Start() error {
	return nil
}

var _ Component = &starterComponent{}

type starterComponent struct {
	startFunc func() error
}

func (s *starterComponent) Start() error {
	return s.startFunc()
}

var _ Component = &closerComponent{}

type closerComponent struct {
	closeFunc func() error
}

func (c *closerComponent) Close() error {
	return c.closeFunc()
}

func (c *closerComponent) Start() error {
	return nil
}
