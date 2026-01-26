package events

import (
	"sync"

	"github.com/OldStager01/cloud-autoscaler/internal/logger"
	"github.com/OldStager01/cloud-autoscaler/pkg/models"
)

type EventBus struct {
	subscribers map[models.EventType][]chan *models.Event
	allChans    []chan *models.Event // Track channels from SubscribeAll
	mu          sync.RWMutex
	bufferSize  int
	closed      bool
}

func NewEventBus(bufferSize int) *EventBus {
	if bufferSize <= 0 {
		bufferSize = 100
	}
	return &EventBus{
		subscribers: make(map[models.EventType][]chan *models.Event),
		allChans:    make([]chan *models.Event, 0),
		bufferSize:  bufferSize,
	}
}

func (b *EventBus) Subscribe(eventType models.EventType) <-chan *models.Event {
	b.mu.Lock()
	defer b.mu.Unlock()

	ch := make(chan *models.Event, b.bufferSize)
	b.subscribers[eventType] = append(b.subscribers[eventType], ch)
	return ch
}

func (b *EventBus) SubscribeAll() <-chan *models.Event {
	b.mu.Lock()
	defer b.mu.Unlock()

	ch := make(chan *models.Event, b.bufferSize)

	for _, eventType := range allEventTypes() {
		b.subscribers[eventType] = append(b.subscribers[eventType], ch)
	}

	b.allChans = append(b.allChans, ch)
	return ch
}

func (b *EventBus) Publish(event *models.Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.closed {
		return
	}

	subscribers := b.subscribers[event.Type]
	for _, ch := range subscribers {
		select {
		case ch <- event: 
		default:
			logger.Warnf("Event channel full, dropping event: %s", event.Type)
		}
	}
}

func (b *EventBus) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return
	}
	b.closed = true

	// Close channels from SubscribeAll (only once each)
	for _, ch := range b.allChans {
		close(ch)
	}

	// Close individual subscriptions (skip if already in allChans)
	closedChans := make(map[chan *models.Event]bool)
	for _, ch := range b.allChans {
		closedChans[ch] = true
	}

	for _, subscribers := range b.subscribers {
		for _, ch := range subscribers {
			if !closedChans[ch] {
				close(ch)
				closedChans[ch] = true
			}
		}
	}

	b.subscribers = make(map[models.EventType][]chan *models.Event)
	b.allChans = nil
}

func allEventTypes() []models.EventType {
	return []models.EventType{
		models.EventTypeMetricCollected,
		models.EventTypeMetricAnalyzed,
		models.EventTypeDecisionMade,
		models.EventTypeScalingStarted,
		models.EventTypeScalingComplete,
		models.EventTypeScalingFailed,
		models.EventTypeServerAdded,
		models.EventTypeServerRemoved,
		models.EventTypeServerActivated,
		models.EventTypeAlert,
		models.EventTypeError,
	}
}