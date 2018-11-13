package physicsnet

import (
	"time"

	"github.com/grinova/classic2d-server/physics"
)

type simulator struct {
	store map[Controller]struct{}
}

func createSimulator() simulator {
	return simulator{store: make(map[Controller]struct{})}
}

func (s simulator) add(body *physics.Body, c Controller) {
	s.store[c] = struct{}{}
	c.OnAddToSimulator(body)
}

func (s simulator) remove(c Controller) {
	delete(s.store, c)
}

func (s simulator) step(d time.Duration) {
	for c := range s.store {
		c.Step(d)
	}
}
