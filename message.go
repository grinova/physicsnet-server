package physicsnet

type message struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

type manageCommand struct {
	ID   string      `json:"id"`
	Data interface{} `json:"data"`
}

type entityRoute struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

type createProps struct {
	ID   string      `json:"id"`
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

type destroyProps struct {
	ID string `json:"id"`
}

// BodyProps - свойства тела
type BodyProps struct {
	ID       string  `json:"id"`
	Position Point   `json:"position"`
	Angle    float64 `json:"angle"`
}

// Point - точка
type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type actorProps struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}
