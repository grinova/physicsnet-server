package physicsnet

import (
	"time"

	"github.com/grinova/classic2d/physics"
)

// Controller - контроллер
type Controller interface {
	Step(body *physics.Body, d time.Duration)
}
