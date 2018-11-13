package physicsnet

import (
	"time"

	"github.com/grinova/classic2d-server/physics"
)

// Controller - контроллер
type Controller interface {
	Step(d time.Duration)
	OnAddToSimulator(body *physics.Body)
}

// BaseController - базовый тип контроллера
type BaseController struct {
	body *physics.Body
}

// OnAddToSimulator - событие добавления контроллена в симулятор
func (c *BaseController) OnAddToSimulator(body *physics.Body) {
	c.body = body
}

// GetBody возвращает ссылку на тело управляемое контроллером
func (c *BaseController) GetBody() *physics.Body {
	return c.body
}
