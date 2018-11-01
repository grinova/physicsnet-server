package physicsnet

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/grinova/classic2d/dynamic"
	"github.com/grinova/classic2d/physics"

	"github.com/gorilla/websocket"
)

const (
	defaultStepDuration = time.Second / 60
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
	sync.RWMutex
	clients
	running      bool
	ch           chan msg
	listener     ServerListener
	world        dynamic.World
	bodies       manager
	synchronizer contextSynchronizer
}

type client struct {
	conn *websocket.Conn
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
	s.clients[id] = c
	s.sync(c)
	go s.client(c, id, s.ch)
	return id, nil
}

// CreateEntity создаёт сущность заданного типа
func (s *Server) CreateEntity(id string, t string, props interface{}) {
	s.bodies.create(createProps{ID: id, Type: t, Data: props})
}

// DestroyEntity уничтожает все сущности с идентификатором id
func (s *Server) DestroyEntity(id string) {
	if item, ok := s.bodies.store[id]; ok {
		s.bodies.destroy(destroyProps{ID: id})
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
	return s.bodies.factory.Registrator
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
	data, ok := data.(map[string]interface{})
	if !ok {
		return false
	}
	return s.listener.OnEventMessage == nil || s.listener.OnEventMessage(s, id, data)
}

func (s *Server) onSystem(id string, data interface{}) bool {
	return s.listener.OnSystemMessage == nil || s.listener.OnSystemMessage(s, id, data)
}

func (s *Server) onStep(d time.Duration) {
	// s.world.Step(d.Seconds() / 1000)
}

func (s *Server) sync(c client) {
	s.synchronizer.context(clientSynchronizer{client: c}, func() {
		s.bodies.sync()
	})
}

func (s *Server) loop(ch <-chan msg) {
	running := true
	past := time.Now()
	ticker := time.NewTicker(defaultStepDuration)
	for running {
		select {
		case m, ok := <-ch:
			if ok {
				s.onMessage(m)
			} else {
				running = false
			}
		case _, ok := <-ticker.C:
			if ok {
				now := time.Now()
				duration := now.Sub(past)
				past = now
				s.onStep(duration)
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
	s.synchronizer = contextSynchronizer{synchronizer: broadcastSynchronizer{client: &s.clients}}
	manageSynchronizer := manageSynchronizer{parent: s.synchronizer}
	bodiesSynchronizer := entitiesSynchronizer{id: "bodies", parent: manageSynchronizer}
	s.bodies = createManager(bodiesSynchronizer)
}
