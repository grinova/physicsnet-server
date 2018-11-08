package physicsnet

type synchronizer interface {
	sync(v interface{})
}

type contextSynchronizer struct {
	synchronizer
}

func (s contextSynchronizer) with(sc synchronizer, f func()) {
	if sc != nil && f != nil {
		backupSynchronizer := s.synchronizer
		s.synchronizer = sc
		f()
		s.synchronizer = backupSynchronizer
	}
}

type clientSynchronizer struct {
	client
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
	s.parent.sync(manageCommand{ID: s.id, Data: v})
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
