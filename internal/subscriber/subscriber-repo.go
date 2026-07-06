package subscriber

import "github.com/dev-khalid/hookwave/internal/events/order"

/**
1. Define interface for Subscriber repository
2. Define methods for subscriber repository
3. Implement the interface in Subscriber Json Repo
*/

type SubscriberRepo interface {
	GetSubscriptions(companyId int, event order.EventType) ([]Subscription, error)
}
