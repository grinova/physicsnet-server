package physicsnet

// Creator - функция создания объекта
type Creator func(props interface{}) interface{}

// Registrator - паттерн регистратора
type Registrator map[string]Creator

// IsRegistered возвращает true если для типа t есть регистрация
func (f Registrator) IsRegistered(t string) bool {
	_, ok := f[t]
	return ok
}

// Register регистрирует функцию создания объекта creator для типа t
func (f Registrator) Register(t string, creator Creator) {
	f[t] = creator
}

// Unregister снимает регистрацию для типа t
func (f Registrator) Unregister(t string) {
	delete(f, t)
}
