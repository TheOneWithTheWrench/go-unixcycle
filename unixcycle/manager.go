package unixcycle

import (
	"syscall"
)

type Manager struct {
	starters []Startable
	stoppers []Stoppable
}

func NewManager(components ...*component) *Manager {
	m := &Manager{}
	for _, c := range components {
		if c.StartFunc != nil {
			m.starters = append(m.starters, c.StartFunc)
		}
		if c.StopFunc != nil {
			m.stoppers = append(m.stoppers, c.StopFunc)
		}
	}
	return m
}

func (m *Manager) Run() syscall.Signal {
	for _, s := range m.starters {
		if err := s.Start(); err != nil {
			return syscall.SIGABRT
		}
	}

	for _, s := range m.stoppers {
		if err := s.Stop(); err != nil {
			return syscall.SIGABRT
		}
	}

	return syscall.SIGTERM
}
