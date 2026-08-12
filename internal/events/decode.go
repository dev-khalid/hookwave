package events

import (
	"encoding/json"
	"fmt"

	"github.com/dev-khalid/hookwave/internal/events/order"
)

// Decode inspects an order event's `type` discriminator in body and unmarshals
// the full body into the matching concrete type from order's Listed event
// types (order.OrderCreatedEvent, order.OrderUpdatedEvent, order.OrderShippedEvent).
// It also returns the shared BaseOrderEvent fields, decoded once, for callers
// that only need id/type/occurred_at/company_id without switching on the
// concrete type themselves.
func Decode(body []byte) (order.BaseOrderEvent, any, error) {
	var base order.BaseOrderEvent
	if err := json.Unmarshal(body, &base); err != nil {
		return order.BaseOrderEvent{}, nil, fmt.Errorf("decode event envelope: %w", err)
	}

	var event any
	switch base.Type {
	case order.OrderCreatedEventType:
		event = &order.OrderCreatedEvent{}
	case order.OrderUpdatedEventType:
		event = &order.OrderUpdatedEvent{}
	case order.OrderShippedEventType:
		event = &order.OrderShippedEvent{}
	default:
		return order.BaseOrderEvent{}, nil, fmt.Errorf("unknown event type %q", base.Type)
	}

	if err := json.Unmarshal(body, event); err != nil {
		return order.BaseOrderEvent{}, nil, fmt.Errorf("decode %s event: %w", base.Type, err)
	}

	return base, event, nil
}
