package physicsnet

type managerItem struct {
	t      string
	props  interface{}
	result interface{}
}

type manager struct {
	factory
	store map[string]managerItem
	create
	destroy
}

func createManager(es *entities) manager {
	return manager{
		store:   make(map[string]managerItem),
		factory: factory{Registrator: make(Registrator)},
		create:  create{parent: es},
		destroy: destroy{parent: es},
	}
}

func (m *manager) Create(id string, t string, data interface{}) interface{} {
	result := m.factory.create(t, data)
	if result != nil {
		props := createProps{ID: id, Type: t, Data: data}
		item := managerItem{t: t, props: props, result: result}
		m.store[id] = item
		m.create.sync(props)
	}
	return result
}

func (m *manager) Destroy(id string) {
	delete(m.store, id)
	props := destroyProps{ID: id}
	m.destroy.sync(props)
}

func (m *manager) get(id string) interface{} {
	if item, ok := m.store[id]; ok {
		return item.result
	}
	return nil
}

func (m *manager) sync() {
	for _, item := range m.store {
		m.create.sync(item.props)
	}
}
