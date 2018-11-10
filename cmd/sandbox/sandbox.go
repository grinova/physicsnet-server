package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/grinova/actors"
	"github.com/grinova/classic2d-server/physics"
	"github.com/grinova/classic2d-server/physics/shapes"
	"github.com/grinova/classic2d-server/vmath"
	physicsnet "github.com/grinova/physicsnet-server"

	"github.com/gorilla/websocket"
)

const (
	shipMaxForce         = 0.1
	shipMaxTorque        = 5
	shipDumpRotationCoef = 0.97
)

type shipController struct {
	thrust float64
	torque float64
}

func (c *shipController) Step(body *physics.Body, d time.Duration) {
	force := vmath.Vec2{X: 0, Y: c.thrust}
	force.Rotate(body.GetRot())
	body.ApplyForce(force.Mul(shipMaxForce))
	body.SetTorque(c.torque * shipMaxTorque)
	body.AngularVelocity *= shipDumpRotationCoef
}

func createShipController(v interface{}) interface{} {
	return &shipController{}
}

func createShipActor(v interface{}) interface{} {
	return physicsnet.Actor{
		OnMessage: func(c physicsnet.Controller, m actors.Message, send actors.Send, spawn actors.Spawn, exit actors.Exit) {
			if ship, ok := c.(*shipController); ok {
				if msg, ok := m.(map[string]interface{}); ok {
					switch msg["type"] {
					case "thrust":
						if thrust, ok := msg["amount"].(float64); ok {
							ship.thrust = thrust
						}
					case "torque":
						if torque, ok := msg["amount"].(float64); ok {
							ship.torque = torque
						}
					}
				}
			}
		},
	}
}

// UserData - данные тела
type UserData struct {
	Type string
	ID   string
}

var shipProps = map[string]physicsnet.BodyCreateProps{
	"ship-a": physicsnet.BodyCreateProps{
		ID:       "ship-a",
		Position: physicsnet.Point{X: -0.5, Y: -0.5},
		Angle:    0,
	},
	"ship-b": physicsnet.BodyCreateProps{
		ID:       "ship-b",
		Position: physicsnet.Point{X: 0.5, Y: 0.5},
		Angle:    0,
	},
}

var (
	port      = flag.String("p", "3000", "port to serve on")
	directory = flag.String("d", ".", "the directory of static file to host")
)

func main() {
	flag.Parse()

	listener := physicsnet.ServerListener{
		OnServerStart: func(s *physicsnet.Server) {
			// TODO: register creators, create black hole
			s.GetBodyRegistrator().Register("arena", func(v interface{}) interface{} {
				bodyDef := physics.BodyDef{Inverse: true}
				body := s.GetWorld().CreateBody(bodyDef)
				shape := shapes.CircleShape{Radius: 1}
				fixtureDef := physics.FixtureDef{Shape: shape, Density: 1}
				body.SetFixture(fixtureDef)
				body.UserData = UserData{Type: "arena", ID: "arena"}
				body.Type = physics.StaticBody
				return body
			})
			s.GetBodyRegistrator().Register("ship", func(v interface{}) interface{} {
				if props, ok := v.(physicsnet.BodyCreateProps); ok {
					RADIUS := 0.05
					bodyDef := physics.BodyDef{
						Position: vmath.Vec2{X: props.Position.X, Y: props.Position.Y},
						Angle:    props.Angle,
					}
					body := s.GetWorld().CreateBody(bodyDef)
					shape := shapes.CircleShape{Radius: RADIUS}
					fixtureDef := physics.FixtureDef{Shape: shape, Density: 1}
					body.SetFixture(fixtureDef)
					body.UserData = UserData{ID: props.ID, Type: "ship"}
					return body
				}
				return nil
			})
			s.GetControllerRegistrator().Register("ship", createShipController)
			s.GetActorRegistrator().Register("ship", createShipActor)
			s.CreateEntity("arena", "arena", physicsnet.BodyCreateProps{})
			log.Println("Server start")
		},
		OnServerStop: func(s *physicsnet.Server) {
			log.Println("Server stop")
		},
		OnClientConnect: func(s *physicsnet.Server, id string) error {
			if props, ok := shipProps[id]; ok {
				s.CreateEntity(id, "ship", props)
				log.Printf("Client connect: id = %s\n", id)
				return nil
			}
			return fmt.Errorf("onClientConnect: didn't find initial properties for ship id `%s`", id)
		},
		OnClientDisconnect: func(s *physicsnet.Server, id string) {
			s.DestroyEntity(id)
			log.Printf("Client disconnect: id = %s\n", id)
		},
		OnEventMessage: func(s *physicsnet.Server, id string, m interface{}) bool {
			log.Printf("Event from %s: %s\n", id, m)
			return true
		},
		OnSystemMessage: func(s *physicsnet.Server, id string, m interface{}) bool {
			log.Printf("System from %s: %s\n", id, m)
			return true
		},
	}
	var server = physicsnet.Server{}
	server.SetListener(listener)
	go server.Loop()

	var upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	http.Handle("/", http.FileServer(http.Dir(*directory)))
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Println("upgrade:", err)
			return
		}
		if _, err := server.Connect(c); err != nil {
			c.Close()
			log.Println("upgrade:", err)
			return
		}
	})
	log.Printf("Serving %s on HTTP port: %s\n", *directory, *port)
	log.Fatal(http.ListenAndServe(":"+*port, nil))
}
