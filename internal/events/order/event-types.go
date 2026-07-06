package order

import "time"

type EventType string

const (
	OrderCreatedEventType EventType = "order.created"
	OrderUpdatedEventType EventType = "order.updated"
	OrderShippedEventType EventType = "order.shipped"
)

var ListedEventTypes = []EventType{
	OrderCreatedEventType,
	OrderUpdatedEventType,
	OrderShippedEventType,
}

type OrderStatus string

const (
	OrderStatusCreated OrderStatus = "created"
	OrderStatusUpdated OrderStatus = "updated"
	OrderStatusShipped OrderStatus = "shipped"
)

type PaymentMethod string

const (
	PaymentMethodCard         PaymentMethod = "card"
	PaymentMethodBankTransfer PaymentMethod = "bank_transfer"
)

type BaseOrderEvent struct {
	ID         string    `json:"id"`
	Type       EventType `json:"type"`
	OccurredAt time.Time `json:"occurred_at"`
	CompanyID  int       `json:"company_id"`
}

type BaseOrderData struct {
	OrderID    string      `json:"order_id"`
	CustomerID string      `json:"customer_id"`
	Status     OrderStatus `json:"status"`
	Currency   string      `json:"currency"`
	Amount     float64     `json:"amount"`
}

/** Order Created Event */

type OrderCreatedEvent struct {
	BaseOrderEvent
	Data OrderCreatedData `json:"data"`
}

type OrderCreatedData struct {
	BaseOrderData
	Status          OrderStatus     `json:"status"`
	Items           []OrderItem     `json:"items"`
	ShippingAddress ShippingAddress `json:"shipping_address"`
}

type OrderItem struct {
	SKU       string  `json:"sku"`
	Name      string  `json:"name"`
	Quantity  int     `json:"quantity"`
	UnitPrice float64 `json:"unit_price"`
}

type ShippingAddress struct {
	Line1      string `json:"line1"`
	City       string `json:"city"`
	PostalCode string `json:"postal_code"`
	Country    string `json:"country"` // NOTE: this should be an enum of valid ISO 3166-1 alpha-2 codes
}

/** Order Updated Event */

type OrderUpdatedEvent struct {
	BaseOrderEvent
	Data OrderUpdatedData `json:"data"`
}

type OrderUpdatedData struct {
	BaseOrderData
	PreviousStatus OrderStatus `json:"previous_status"`
	Changes        Changes     `json:"changes"`
}

type Changes struct {
	Status  StatusChange `json:"status"`
	Payment PaymentInfo  `json:"payment"`
}

type StatusChange struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type PaymentInfo struct {
	Method        PaymentMethod `json:"method"`
	TransactionID string        `json:"transaction_id"`
	PaidAt        time.Time     `json:"paid_at"`
}

/** Order Shipped Event */

type OrderShippedEvent struct {
	BaseOrderEvent
	Data OrderShippedData `json:"data"`
}

type OrderShippedData struct {
	BaseOrderData
	Shipment Shipment `json:"shipment"`
}

type Shipment struct {
	Carrier           string      `json:"carrier"`
	TrackingNumber    string      `json:"tracking_number"`
	TrackingURL       string      `json:"tracking_url"`
	ShippedAt         time.Time   `json:"shipped_at"`
	EstimatedDelivery time.Time   `json:"estimated_delivery"`
	Items             []OrderItem `json:"items"`
}
