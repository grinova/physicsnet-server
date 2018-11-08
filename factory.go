package physicsnet

type factory struct {
	Registrator
}

func (f factory) create(t string, data interface{}) interface{} {
	creator, ok := f.Registrator[t]
	if !ok {
		return nil
	}
	return creator(data)
}
