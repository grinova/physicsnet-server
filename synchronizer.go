package physicsnet

import "github.com/grinova/classic2d-server/physics"

type synchronizer interface {
	sync(v interface{})
}

type context struct {
	context synchronizer
}

func (s *context) sync(v interface{}) {
	if s.context != nil {
		s.context.sync(v)
	}
}

func (s *context) with(context synchronizer, f func()) {
	if f != nil {
		backup := s.context
		s.context = context
		f()
		s.context = backup
	}
}

type broadcast struct {
	clients
}

func (s *broadcast) sync(v interface{}) {
	for _, client := range s.clients {
		client.sync(v)
	}
}

type except struct {
	*broadcast
	exceptID string
}

func (s *except) sync(v interface{}) {
	for id, client := range s.clients {
		if id != s.exceptID {
			client.sync(v)
		}
	}
}

type manage struct {
	parent synchronizer
}

func (s *manage) sync(v interface{}) {
	s.parent.sync(message{Type: "manage", Data: v})
}

type entities struct {
	id     string
	parent *manage
}

func (s *entities) sync(v interface{}) {
	s.parent.sync(route{ID: s.id, Data: v})
}

type create struct {
	parent *entities
}

func (s *create) sync(v interface{}) {
	s.parent.sync(entityRoute{Type: "create", Data: v})
}

type destroy struct {
	parent *entities
}

func (s *destroy) sync(v interface{}) {
	s.parent.sync(entityRoute{Type: "destroy", Data: v})
}

type synchronize struct {
	parent *broadcast
}

func (s *synchronize) sync(v interface{}) {
	s.parent.sync(message{Type: "sync", Data: v})
}

type bodies struct {
	parent  *synchronize
	manager *manager
}

func (s *bodies) sync() {
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
	s.parent.sync(route{ID: "default", Data: bodiesSync})
}

type event struct {
	parent synchronizer
}

func (s *event) sync(v interface{}) {
	s.parent.sync(message{Type: "event", Data: v})
}
