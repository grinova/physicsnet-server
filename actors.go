package physicsnet

import (
	actrs "github.com/grinova/actors"
)

// Spawn - функция создания актора
type Spawn func(t string, props interface{}) (actrs.ActorID, bool)

// Actor - актор
type Actor interface {
	OnInit(controller Controller, selfID actrs.ActorID, send actrs.Send, spawn Spawn, exit actrs.Exit)
	OnMessage(controller Controller, message actrs.Message, send actrs.Send, spawn Spawn, exit actrs.Exit)
}

type actorOwner struct {
	actor      Actor
	controller Controller
	spawn      Spawn
}

func (ao *actorOwner) OnInit(selfID actrs.ActorID, send actrs.Send, spawn actrs.Spawn, exit actrs.Exit) {
	ao.actor.OnInit(ao.controller, selfID, send, ao.spawn, exit)
}

func (ao *actorOwner) OnMessage(message actrs.Message, send actrs.Send, spawn actrs.Spawn, exit actrs.Exit) {
	ao.actor.OnMessage(ao.controller, message, send, ao.spawn, exit)
}

type createActorController func(id string, t string, bodyCreateProps interface{}) (Actor, Controller, bool)

type actors struct {
	*actrs.Actors
	createActorController createActorController
}

func createActors(idGenerator actrs.IDGenerator, createActorController createActorController) actors {
	a := actrs.New(actrs.Props{RootIDGenerator: idGenerator})
	return actors{Actors: &a, createActorController: createActorController}
}

func (a *actors) spawn(controller Controller, actor Actor) (actrs.ActorID, bool) {
	return a.Spawn(func(id actrs.ActorID) (actrs.Actor, bool) {
		return &actorOwner{
			actor:      actor,
			controller: controller,
			spawn:      a.spawnActor,
		}, true
	})
}

func (a *actors) spawnActor(t string, props interface{}) (actrs.ActorID, bool) {
	return a.Spawn(func(id actrs.ActorID) (actrs.Actor, bool) {
		if actor, controller, ok := a.createActorController(id, t, props); ok {
			return &actorOwner{
				actor:      actor,
				controller: controller,
				spawn:      a.spawnActor,
			}, true
		}
		return nil, false
	})
}
