package physicsnet

type managerItem struct {
	t      string
	props  interface{}
	result interface{}
}

type manager struct {
	factory
	store map[string]managerItem
	createSynchronizer
	destroySynchronizer
}

func createManager(es entitiesSynchronizer) manager {
	return manager{
		store:               make(map[string]managerItem),
		factory:             factory{Registrator: make(Registrator)},
		createSynchronizer:  createSynchronizer{parent: es},
		destroySynchronizer: destroySynchronizer{parent: es},
	}
}

func (m *manager) create(id string, t string, data interface{}) interface{} {
	result := m.factory.create(t, data)
	if result != nil {
		props := createProps{ID: id, Type: t, Data: data}
		item := managerItem{t: t, props: props, result: result}
		m.store[id] = item
		m.createSynchronizer.sync(item.props)
	}
	return result
}

func (m *manager) destroy(id string) {
	delete(m.store, id)
	props := destroyProps{ID: id}
	m.destroySynchronizer.sync(props)
}

func (m *manager) get(id string) interface{} {
	if item, ok := m.store[id]; ok {
		return item.result
	}
	return nil
}

func (m *manager) sync() {
	for _, item := range m.store {
		m.createSynchronizer.sync(item.props)
	}
}
