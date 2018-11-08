package physicsnet

import (
	actrs "github.com/grinova/actors"
)

type customIDGenerator struct {
	actrs.NumericIDGenerator
	id string
}

func (g *customIDGenerator) NewID() string {
	if g.id == "" {
		return g.NumericIDGenerator.NewID()
	}
	id := g.id
	g.id = ""
	return id
}
