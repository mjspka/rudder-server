//go:generate mockgen -destination=../mocks/utils/mock_pubsub.go -package=utils github.com/rudderlabs/rudder-server/utils PublishSubscriber

package utils

import (
	"sync"
)

type DataEvent struct {
	Data  interface{}
	Topic string
	Wait  *sync.WaitGroup
}

// DataChannel is a channel which can accept an DataEvent
type DataChannel chan DataEvent

type PublishSubscriber interface {
	Publish(topic string, data interface{})
	Subscribe(topic string, ch DataChannel)
}

// EventBus stores the information about subscribers interested for a particular topic
type EventBus struct {
	lastEventMutex sync.RWMutex
	lastEvent      map[string]*DataEvent

	subscribersMutex sync.RWMutex
	subscribers      map[string]publisherSlice
}

func (eb *EventBus) Publish(topic string, data interface{}) {
	eb.subscribersMutex.RLock()
	defer eb.subscribersMutex.RUnlock()
	eb.lastEventMutex.Lock()
	defer eb.lastEventMutex.Unlock()

	evt := &DataEvent{Data: data, Topic: topic}
	if eb.lastEvent == nil {
		eb.lastEvent = map[string]*DataEvent{}
	}
	eb.lastEvent[topic] = evt

	if publishers, found := eb.subscribers[topic]; found {
		for _, publisher := range publishers {
			publisher.publish(*evt)
		}
	}

}

func (eb *EventBus) Subscribe(topic string, ch DataChannel) {
	eb.subscribersMutex.Lock()
	defer eb.subscribersMutex.Unlock()
	eb.lastEventMutex.RLock()
	defer eb.lastEventMutex.RUnlock()

	p := &publisher{channel: ch}
	if prev, found := eb.subscribers[topic]; found {
		eb.subscribers[topic] = append(prev, p)
	} else {
		if eb.subscribers == nil {
			eb.subscribers = map[string]publisherSlice{}
		}
		eb.subscribers[topic] = publisherSlice{p}
	}
	if eb.lastEvent[topic] != nil {
		p.publish(*eb.lastEvent[topic])
	}
}

type publisherSlice []*publisher
type publisher struct {
	channel chan DataEvent

	lastValueLock sync.Mutex
	lastValue     *DataEvent

	startedLock sync.Mutex
	started     bool
}

func (r *publisher) publish(data DataEvent) {

	r.lastValueLock.Lock()
	defer r.lastValueLock.Unlock()
	r.startedLock.Lock()
	defer r.startedLock.Unlock()

	// update last value
	if r.lastValue != nil {
		// TODO: log a warning about slow consumer
	}
	r.lastValue = &data

	// start publish loop if not started
	if !r.started {
		go r.startLoop()
		r.started = true
	}
}

func (r *publisher) startLoop() {

	for r.lastValue != nil {
		r.lastValueLock.Lock()
		v := *r.lastValue
		var wg sync.WaitGroup
		v.Wait = &wg
		r.lastValue = nil
		r.lastValueLock.Unlock()
		wg.Add(1)
		r.channel <- v
		// wg.Wait()
	}
	r.startedLock.Lock()
	r.started = false
	r.startedLock.Unlock()
}
