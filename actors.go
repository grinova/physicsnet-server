package physicsnet

import (
	actrs "github.com/grinova/actors"
)

// Actor - актор
type Actor struct {
	OnInit    func(controller Controller, selfID actrs.ActorID, send actrs.Send, spawn actrs.Spawn, exit actrs.Exit)
	OnMessage func(controller Controller, message actrs.Message, send actrs.Send, spawn actrs.Spawn, exit actrs.Exit)
}

type actors struct {
	*actrs.Actors
}

func createActors(idGenerator actrs.IDGenerator) actors {
	a := actrs.New(actrs.Props{RootIDGenerator: idGenerator})
	return actors{Actors: &a}
}

func (a *actors) send(id actrs.ActorID, message actrs.Message) {
	a.Send(id, message)
}

func (a *actors) spawn(controller Controller, actor Actor) (actrs.ActorID, bool) {
	id, ok := a.Spawn(func(id actrs.ActorID) (actrs.Actor, bool) {
		a := actrs.Actor{}
		if actor.OnInit != nil {
			a.OnInit = func(selfID actrs.ActorID, send actrs.Send, spawn actrs.Spawn, exit actrs.Exit) {
				actor.OnInit(controller, selfID, send, spawn, exit)
			}
		}
		if actor.OnMessage != nil {
			a.OnMessage = func(message actrs.Message, send actrs.Send, spawn actrs.Spawn, exit actrs.Exit) {
				actor.OnMessage(controller, message, send, spawn, exit)
			}
		}
		return a, true
	})
	if !ok {
		return "", false
	}
	return id, true
}
