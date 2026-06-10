// Package events предоставляет простой in-memory pub/sub-брокер для рассылки
// уведомлений подключённым SSE-клиентам (например, об изменении профилей).
package events

import "sync"

// Broker рассылает строковые события всем активным подписчикам.
type Broker struct {
	mu     sync.Mutex
	subs   map[int]chan string
	nextID int
}

// NewBroker создаёт пустой брокер.
func NewBroker() *Broker {
	return &Broker{subs: make(map[int]chan string)}
}

// Subscribe регистрирует нового подписчика и возвращает его идентификатор и канал.
// Канал буферизован: медленный подписчик не блокирует Publish.
func (b *Broker) Subscribe() (int, <-chan string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	id := b.nextID
	b.nextID++
	ch := make(chan string, 8)
	b.subs[id] = ch
	return id, ch
}

// Unsubscribe удаляет подписчика и закрывает его канал.
func (b *Broker) Unsubscribe(id int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if ch, ok := b.subs[id]; ok {
		delete(b.subs, id)
		close(ch)
	}
}

// Publish неблокирующе рассылает событие всем подписчикам. Если буфер канала
// подписчика заполнен, событие для него пропускается — клиент всё равно
// перезапросит актуальное состояние при следующем событии.
func (b *Broker) Publish(event string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, ch := range b.subs {
		select {
		case ch <- event:
		default:
		}
	}
}
