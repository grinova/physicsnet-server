package physicsnet

import "github.com/grinova/classic2d-server/physics"

type synchronizer interface {
	sync(v interface{})
}

type contextSynchronizer struct {
	synchronizer
}

func (s *contextSynchronizer) sync(v interface{}) {
	if s.synchronizer != nil {
		s.synchronizer.sync(v)
	}
}

func (s *contextSynchronizer) with(sc synchronizer, f func()) {
	if f != nil {
		backupSynchronizer := s.synchronizer
		s.synchronizer = sc
		f()
		s.synchronizer = backupSynchronizer
	}
}

type clientSynchronizer struct {
	*Client
}

func (s clientSynchronizer) sync(v interface{}) {
	s.conn.WriteJSON(v)
}

type broadcastSynchronizer struct {
	client *clients
}

func (s broadcastSynchronizer) sync(v interface{}) {
	for _, client := range *s.client {
		client.conn.WriteJSON(v)
	}
}

type exceptSynchronizer struct {
	broadcastSynchronizer
	exceptID string
}

func (s exceptSynchronizer) sync(v interface{}) {
	for id, client := range *s.client {
		if id != s.exceptID {
			client.conn.WriteJSON(v)
		}
	}
}

type manageSynchronizer struct {
	parent synchronizer
}

func (s manageSynchronizer) sync(v interface{}) {
	if s.parent != nil {
		s.parent.sync(message{Type: "manage", Data: v})
	}
}

type entitiesSynchronizer struct {
	id     string
	parent manageSynchronizer
}

func (s entitiesSynchronizer) sync(v interface{}) {
	s.parent.sync(commandProps{ID: s.id, Data: v})
}

type createSynchronizer struct {
	parent entitiesSynchronizer
}

func (s createSynchronizer) sync(v interface{}) {
	s.parent.sync(entityRoute{Type: "create", Data: v})
}

type destroySynchronizer struct {
	parent entitiesSynchronizer
}

func (s destroySynchronizer) sync(v interface{}) {
	s.parent.sync(entityRoute{Type: "destroy", Data: v})
}

type syncSynchronizer struct {
	parent broadcastSynchronizer
}

func (s syncSynchronizer) sync(v interface{}) {
	s.parent.sync(message{Type: "sync", Data: v})
}

type bodiesSynchronizer struct {
	parent  syncSynchronizer
	manager *manager
}

func (s bodiesSynchronizer) sync() {
	bodiesSync := make(bodiesSync)
	for id, item := range s.manager.store {
		if body, ok := item.result.(*physics.Body); ok {
			position := body.GetPosition()
			angle := body.GetAngle()
			props := bodySyncProps{
				Position:        Point{X: position.X, Y: position.Y},
				Angle:           angle,
				LinearVelocity:  Point{X: body.LinearVelocity.X, Y: body.LinearVelocity.Y},
				AngularVelocity: body.AngularVelocity,
			}
			bodiesSync[id] = props
		}
	}
	s.parent.sync(syncProps{ID: "default", Data: bodiesSync})
}

type eventSynchronizer struct {
	parent synchronizer
}

func (s eventSynchronizer) sync(v interface{}) {
	s.parent.sync(message{Type: "event", Data: v})
}
