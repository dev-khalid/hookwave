package subscriber

import (
	"log"

	"github.com/dev-khalid/hookwave/internal/events/order"
	"github.com/google/uuid"
)

type Subscription struct {
	ID        uuid.UUID         `json:"id"`
	Events    []order.EventType `json:"events"`
	URL       string            `json:"url"`
	Method    string            `json:"method"`
	CompanyID int               `json:"company_id"`
}

func (s *Subscription) HasEvent(event order.EventType) bool {
	for _, e := range s.Events {
		if e == event {
			return true
		}
	}
	return false
}

type SubscriberService struct {
	SubscriberRepo SubscriberRepo
}

var NewSubscriberService = func() (SubscriberService, error) {
	repo, err := NewSubscriberJsonRepo()
	if err != nil {
		log.Fatalf("failed to create subscriber repo: %v", err)
		return SubscriberService{}, err
	}

	return SubscriberService{
		SubscriberRepo: repo,
	}, nil
}
