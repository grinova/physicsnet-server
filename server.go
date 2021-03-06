package physicsnet

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/grinova/classic2d-server/dynamic"
	"github.com/grinova/classic2d-server/physics"

	"github.com/gorilla/websocket"
)

const (
	defaultStepDuration = time.Second / 60
	defaultSyncDuration = time.Second / 5
)

// ServerListener - интерфейс событий сервера
type ServerListener struct {
	OnServerStart      func(s *Server)
	OnServerStop       func(s *Server)
	OnClientConnect    func(s *Server, id string, client *Client) error
	OnClientDisconnect func(s *Server, id string)
	OnEventMessage     func(s *Server, id string, data interface{}) bool
	OnSystemMessage    func(s *Server, id string, data interface{}) bool
}

// ServerProps - свойства сервера
type ServerProps struct {
	Synchronization bool
	NewID           func() (string, error)
}

// Server - сервер физики
type Server struct {
	Props ServerProps
	sync.RWMutex
	clients            clients
	running            bool
	ch                 chan msg
	listener           ServerListener
	world              dynamic.World
	bodiesIDs          map[*physics.Body]string
	bodiesManager      manager
	controllersManager manager
	actorsManager      manager
	actors             actors
	idGenerator        customIDGenerator
	simulator          simulator
	context            *context
	broadcast          *broadcast
	event              *event
	bodies             *bodies
}

type clients map[string]*Client

type msg struct {
	id   string
	data interface{}
}

// Close завершает сервер
func (s *Server) Close() {
	defer s.Unlock()
	s.Lock()
	if !s.running {
		return
	}
	for id := range s.clients {
		s.disconnect(id)
	}
	close(s.ch)
}

// Connect подключает нового пользователя
func (s *Server) Connect(conn *websocket.Conn) (string, error) {
	defer s.Unlock()
	s.Lock()
	if s.Props.NewID == nil {
		s.Props.NewID = createIDGenerator()
	}
	id, err := s.Props.NewID()
	if err != nil {
		return "", fmt.Errorf("connect: %s", err)
	}
	client := &Client{
		conn: conn,
		except: &except{
			broadcast: s.broadcast,
			exceptID:  id,
		},
	}
	if s.listener.OnClientConnect != nil {
		if err := s.listener.OnClientConnect(s, id, client); err != nil {
			return "", fmt.Errorf("connect: %s", err)
		}
	}
	s.clients[id] = client
	s.sync(client)
	go s.client(client, id, s.ch)
	return id, nil
}

// CreateEntity создаёт сущность заданного типа
func (s *Server) CreateEntity(id string, t string, bodyCreateProps interface{}) {
	if actor, controller, ok := s.createActorController(id, t, bodyCreateProps); ok {
		s.idGenerator.id = id
		s.actors.spawn(controller, actor)
	}
}

// DestroyEntity уничтожает все сущности с идентификатором id
func (s *Server) DestroyEntity(id string) {
	s.actorsManager.Destroy(id)
	if controller, ok := s.controllersManager.get(id).(Controller); ok {
		s.simulator.remove(controller)
		s.controllersManager.Destroy(id)
	}
	if body, ok := s.bodiesManager.get(id).(*physics.Body); ok {
		delete(s.bodiesIDs, body)
		s.world.DestroyBody(body)
	}
	s.bodiesManager.Destroy(id)
}

// DestroyBody уничтожает тело и все соответствующие ему сущности
func (s *Server) DestroyBody(body *physics.Body) {
	if id, ok := s.bodiesIDs[body]; ok {
		s.DestroyEntity(id)
	}
}

// DestroyContact уничтожает контакт
func (s *Server) DestroyContact(contact *dynamic.Contact) {
	s.world.GetContactManager().Destroy(contact)
}

// Disconnect отключает клиента с идентификатором id
func (s *Server) Disconnect(id string) {
	defer s.Unlock()
	s.Lock()
	s.disconnect(id)
}

// GetBody возвращает тело по идентификатору
func (s *Server) GetBody(id string) *physics.Body {
	if body, ok := s.bodiesManager.get(id).(*physics.Body); ok {
		return body
	}
	return nil
}

// GetClient возвращает клиента по идентиикатору
func (s *Server) GetClient(id string) *Client {
	return s.clients[id]
}

// GetBodyRegistrator возвращает регистратор для тел
func (s *Server) GetBodyRegistrator() Registrator {
	return s.bodiesManager.factory.Registrator
}

// GetControllerRegistrator возвращает регистратор для тел
func (s *Server) GetControllerRegistrator() Registrator {
	return s.controllersManager.factory.Registrator
}

// GetActorRegistrator возвращает регистратор для тел
func (s *Server) GetActorRegistrator() Registrator {
	return s.actorsManager.factory.Registrator
}

// GetWorld возвращает мир
func (s *Server) GetWorld() *dynamic.World {
	return &s.world
}

// Loop - основной цикл обработки сообщений клиентов
func (s *Server) Loop() {
	if !s.start() {
		return
	}
	s.loop(s.ch)
	s.stop()
}

// SetListener устанавливает объект обработчик серверных событий
func (s *Server) SetListener(listener ServerListener) {
	s.listener = listener
}

func (s *Server) client(c *Client, id string, ch chan<- msg) {
	defer s.Disconnect(id)
	for {
		var data interface{}
		err := c.conn.ReadJSON(&data)
		if err != nil {
			log.Println("ReadJSON:", err)
			return
		}
		ch <- msg{id: id, data: data}
	}
}

// FIXME: disconnect вызывается дважды для каждого подключения при закрытии сервера
func (s *Server) disconnect(id string) {
	if slot, ok := s.clients[id]; ok {
		slot.conn.Close()
		delete(s.clients, id)
		if s.listener.OnClientDisconnect != nil {
			s.listener.OnClientDisconnect(s, id)
		}
	}
}

func (s *Server) onMessage(m msg) bool {
	defer s.RUnlock()
	s.RLock()
	data, ok := m.data.(map[string]interface{})
	if !ok {
		return false
	}
	if t, ok := data["type"]; ok {
		switch t {
		case "event":
			return s.onEvent(m.id, data["data"])
		case "system":
			return s.onSystem(m.id, data["data"])
		}
	}
	return false
}

func (s *Server) onEvent(id string, data interface{}) bool {
	if data, ok := data.(map[string]interface{}); ok {
		if s.listener.OnEventMessage != nil && !s.listener.OnEventMessage(s, id, data) {
			return false
		}
		if id, ok := data["id"].(string); ok {
			s.actors.Send(id, data["data"])
		}
		if client, ok := s.clients[id]; ok {
			s.context.with(client.except, func() {
				s.event.sync(data)
			})
		}
	}
	return true
}

func (s *Server) onSystem(id string, data interface{}) bool {
	return s.listener.OnSystemMessage == nil || s.listener.OnSystemMessage(s, id, data)
}

func (s *Server) onStep(d time.Duration) {
	s.world.ClearForces()
	s.simulator.step(d)
	s.world.Step(d)
}

func (s *Server) onSync() {
	if s.Props.Synchronization {
		s.bodies.sync()
	}
}

func (s *Server) createActorController(id string, t string, props interface{}) (Actor, Controller, bool) {
	if body, ok := s.bodiesManager.Create(id, t, props).(*physics.Body); ok {
		s.bodiesIDs[body] = id
		if controller, ok := s.controllersManager.Create(id, t, nil).(Controller); ok {
			s.simulator.add(body, controller)
			if actor, ok := s.actorsManager.Create(id, t, nil).(Actor); ok {
				return actor, controller, true
			}
		}
	}
	return nil, nil, false
}

func (s *Server) createActorControllerSilent(id string, t string, props interface{}) (a Actor, c Controller, ok bool) {
	s.context.with(nil, func() {
		a, c, ok = s.createActorController(id, t, props)
	})
	return a, c, ok
}

func (s *Server) sync(client *Client) {
	s.context.with(client, func() {
		s.bodiesManager.sync()
		s.controllersManager.sync()
		s.actorsManager.sync()
	})
}

func (s *Server) loop(ch <-chan msg) {
	running := true
	past := time.Now()
	stepTicker := time.NewTicker(defaultStepDuration)
	syncTicker := time.NewTicker(defaultSyncDuration)
	defer stepTicker.Stop()
	defer syncTicker.Stop()
	for running {
		select {
		case m, ok := <-ch:
			if ok {
				s.onMessage(m)
			} else {
				running = false
			}
		case _, ok := <-stepTicker.C:
			if ok {
				now := time.Now()
				duration := now.Sub(past)
				past = now
				s.onStep(duration)
			}
		case _, ok := <-syncTicker.C:
			if ok {
				s.onSync()
			}
		}
	}
}

func (s *Server) start() bool {
	defer s.Unlock()
	s.Lock()
	if s.running {
		return false
	}
	s.reset()
	if s.listener.OnServerStart != nil {
		s.listener.OnServerStart(s)
	}
	return true
}

func (s *Server) stop() {
	defer s.Unlock()
	s.Lock()
	s.running = false
	if s.listener.OnServerStop != nil {
		s.listener.OnServerStop(s)
	}
}

func (s *Server) reset() {
	s.clients = make(clients)
	s.ch = make(chan msg)
	s.running = true
	s.world = dynamic.CreateWorld()
	s.bodiesIDs = make(map[*physics.Body]string)
	s.broadcast = &broadcast{clients: s.clients}
	s.context = &context{context: s.broadcast}
	s.event = &event{parent: s.context}
	manage := &manage{parent: s.context}
	s.bodiesManager = createManager(&entities{id: "bodies", parent: manage})
	s.controllersManager = createManager(&entities{id: "controllers", parent: manage})
	s.actorsManager = createManager(&entities{id: "actors", parent: manage})
	s.actors = createActors(&s.idGenerator, s.createActorControllerSilent)
	s.simulator = createSimulator()
	s.bodies = &bodies{parent: &synchronize{parent: s.broadcast}, manager: &s.bodiesManager}
}
