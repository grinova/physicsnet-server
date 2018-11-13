package physicsnet

// IDGenerator - генератор идентификаторов
type IDGenerator func() (string, error)

func createIDGenerator() IDGenerator {
	n := 0
	return func() (string, error) {
		id := string(n)
		n++
		return id, nil
	}
}
