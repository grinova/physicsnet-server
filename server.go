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
	defaultSyncDuration = time.Second / 10
)

// ServerListener - интерфейс событий сервера
type ServerListener struct {
	OnServerStart      func(s *Server)
	OnServerStop       func(s *Server)
	OnClientConnect    func(s *Server, id string) error
	OnClientDisconnect func(s *Server, id string)
	OnEventMessage     func(s *Server, id string, data interface{}) bool
	OnSystemMessage    func(s *Server, id string, data interface{}) bool
}

// Server - сервер физики
type Server struct {
	Synchronization bool
	sync.RWMutex
	clients
	running            bool
	ch                 chan msg
	listener           ServerListener
	world              dynamic.World
	bodiesManager      manager
	controllersManager manager
	actorsManager      manager
	actors             actors
	synchronizer       contextSynchronizer
	idGenerator        customIDGenerator
	simulator
	broadcastSynchronizer broadcastSynchronizer
	eventSynchronizer     eventSynchronizer
	bodiesSynchronizer
}

type client struct {
	conn               *websocket.Conn
	synchronizer       clientSynchronizer
	exceptSynchronizer exceptSynchronizer
}

type clients map[string]client

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
	id, err := s.genNewID()
	if err != nil {
		return "", fmt.Errorf("connect: %s", err)
	}
	if s.listener.OnClientConnect != nil {
		if err := s.listener.OnClientConnect(s, id); err != nil {
			return "", fmt.Errorf("connect: %s", err)
		}
	}
	c := client{conn: conn}
	c.synchronizer = clientSynchronizer{client: &c}
	c.exceptSynchronizer = exceptSynchronizer{
		broadcastSynchronizer: s.broadcastSynchronizer,
		exceptID:              id,
	}
	s.clients[id] = c
	s.sync(c)
	go s.client(c, id, s.ch)
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
	s.actorsManager.destroy(id)
	if controller, ok := s.controllersManager.get(id).(Controller); ok {
		s.controllersManager.destroy(id)
		s.simulator.remove(controller)
	}
	s.bodiesManager.destroy(id)
	if item, ok := s.bodiesManager.store[id]; ok {
		if body, ok := item.result.(*physics.Body); ok {
			s.world.DestroyBody(body)
		}
	}
}

// Disconnect отключает клиента с идентификатором id
func (s *Server) Disconnect(id string) {
	defer s.Unlock()
	s.Lock()
	s.disconnect(id)
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

func (s *Server) client(c client, id string, ch chan<- msg) {
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

func (s *Server) genNewID() (string, error) {
	// TODO: Вынести генерацию идентификаторов наружу библиотеки
	for i := 'a'; i < 'z'; i++ {
		id := "ship-" + string(i)
		exist := false
		for shipID := range s.clients {
			if id == shipID {
				exist = true
				break
			}
		}
		if !exist {
			return id, nil
		}
	}
	return "", fmt.Errorf("genNewID: can't generate new id")
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
		if c, ok := s.clients[id]; ok {
			s.synchronizer.with(c.exceptSynchronizer, func() {
				s.eventSynchronizer.sync(data)
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
	if s.Synchronization {
		s.bodiesSynchronizer.sync()
	}
}

func (s *Server) createActorController(id string, t string, props interface{}) (Actor, Controller, bool) {
	if body, ok := s.bodiesManager.create(id, t, props).(*physics.Body); ok {
		if controller, ok := s.controllersManager.create(id, t, nil).(Controller); ok {
			s.simulator.add(body, controller)
			if actor, ok := s.actorsManager.create(id, t, nil).(Actor); ok {
				return actor, controller, true
			}
		}
	}
	return nil, nil, false
}

func (s *Server) createActorControllerSilent(id string, t string, props interface{}) (a Actor, c Controller, ok bool) {
	s.synchronizer.with(nil, func() {
		a, c, ok = s.createActorController(id, t, props)
	})
	return a, c, ok
}

func (s *Server) sync(c client) {
	s.synchronizer.with(c.synchronizer, func() {
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
	s.clients = make(map[string]client)
	s.ch = make(chan msg)
	s.running = true
	s.world = dynamic.CreateWorld()
	s.broadcastSynchronizer = broadcastSynchronizer{client: &s.clients}
	s.synchronizer = contextSynchronizer{synchronizer: s.broadcastSynchronizer}
	s.eventSynchronizer = eventSynchronizer{parent: &s.synchronizer}
	manageSynchronizer := manageSynchronizer{parent: &s.synchronizer}
	s.bodiesManager = createManager(entitiesSynchronizer{id: "bodies", parent: manageSynchronizer})
	s.controllersManager = createManager(entitiesSynchronizer{id: "controllers", parent: manageSynchronizer})
	s.actorsManager = createManager(entitiesSynchronizer{id: "actors", parent: manageSynchronizer})
	s.actors = createActors(&s.idGenerator, s.createActorControllerSilent)
	s.simulator = createSimulator()
	ss := syncSynchronizer{parent: s.broadcastSynchronizer}
	s.bodiesSynchronizer = bodiesSynchronizer{parent: ss, manager: &s.bodiesManager}
}
