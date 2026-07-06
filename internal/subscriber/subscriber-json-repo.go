package subscriber

import (
	"encoding/json"
	"log"
	"os"

	"github.com/dev-khalid/hookwave/internal/events/order"
)

type SubscriberJsonRepo struct {
	SubscriptionConfigs []Subscription
}

func (s *SubscriberJsonRepo) GetSubscriptions(companyId int, event order.EventType) ([]Subscription, error) {
	var subscriptions []Subscription
	for _, sub := range s.SubscriptionConfigs {
		if sub.CompanyID == companyId {
			if sub.HasEvent(event) {
				subscriptions = append(subscriptions, sub)
			}
		}
	}
	return subscriptions, nil
}

func NewSubscriberJsonRepo() (SubscriberRepo, error) {
	// Read json file here.
	rawJson, err := os.ReadFile("./configs/subscriptions.json")
	if err != nil {
		log.Printf("subscriber: failed to read subscriptions config: %v", err)
		return nil, err
	}

	var jsonData = []Subscription{}
	if err := json.Unmarshal(rawJson, &jsonData); err != nil {
		log.Printf("subscriber: failed to parse subscriptions config: %v", err)
		return nil, err
	}

	return &SubscriberJsonRepo{
		SubscriptionConfigs: jsonData,
	}, nil
}
