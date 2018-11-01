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

func (m *manager) create(props createProps) interface{} {
	result := m.factory.create(props.Type, props.Data)
	if result != nil {
		item := managerItem{t: props.Type, props: props, result: result}
		m.store[props.ID] = item
		m.createSynchronizer.sync(item.props)
	}
	return result
}

func (m *manager) destroy(props destroyProps) {
	delete(m.store, props.ID)
	m.destroySynchronizer.sync(props)
}

func (m *manager) sync() {
	for _, item := range m.store {
		m.createSynchronizer.sync(item.props)
	}
}
