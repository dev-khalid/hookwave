package events

import "github.com/dev-khalid/hookwave/internal/events/order"

// ListedEventTypes is a type constraint restricting publishable domain events to known concrete types.
type ListedEventTypes interface {
	*order.OrderCreatedEvent | *order.OrderUpdatedEvent | *order.OrderShippedEvent
}
