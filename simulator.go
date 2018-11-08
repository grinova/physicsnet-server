package physicsnet

import (
	"time"

	"github.com/grinova/classic2d/physics"
)

type simulator struct {
	store map[Controller]*physics.Body
}

func (s simulator) add(body *physics.Body, c Controller) {
	s.store[c] = body
}

func (s simulator) remove(c Controller) {
	delete(s.store, c)
}

func (s simulator) step(d time.Duration) {
	for c, body := range s.store {
		c.Step(body, d)
	}
}
