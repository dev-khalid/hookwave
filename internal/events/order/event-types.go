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

var ListedOrderStatuses = []OrderStatus{
	OrderStatusCreated,
	OrderStatusUpdated,
	OrderStatusShipped,
}

type PaymentMethod string

const (
	PaymentMethodCard         PaymentMethod = "card"
	PaymentMethodBankTransfer PaymentMethod = "bank_transfer"
)

var ListedPaymentMethods = []PaymentMethod{
	PaymentMethodCard,
	PaymentMethodBankTransfer,
}

type BaseOrderEvent struct {
	ID         string    `json:"id" validate:"required,uuid4"`
	Type       EventType `json:"type" validate:"required,EventType"`
	OccurredAt time.Time `json:"occurred_at" validate:"required"`
	CompanyID  int       `json:"company_id" validate:"required,gt=0"`
}

type BaseOrderData struct {
	OrderID    string      `json:"order_id" validate:"required,uuid4"`
	CustomerID int         `json:"customer_id" validate:"required,numeric,gt=0"`
	Status     OrderStatus `json:"status" validate:"required,OrderStatus"`
	Currency   string      `json:"currency" validate:"required,iso4217"`
	Amount     float64     `json:"amount" validate:"gte=0"`
}

/** Order Created Event */

type OrderCreatedEvent struct {
	BaseOrderEvent
	Data OrderCreatedData `json:"data" validate:"required"`
}

type OrderCreatedData struct {
	BaseOrderData
	Items           []OrderItem     `json:"items" validate:"required,min=1,dive"`
	ShippingAddress ShippingAddress `json:"shipping_address" validate:"required"`
}

type OrderItem struct {
	SKU       string  `json:"sku" validate:"required"`
	Name      string  `json:"name" validate:"required"`
	Quantity  int     `json:"quantity" validate:"required,gt=0"`
	UnitPrice float64 `json:"unit_price" validate:"gte=0"`
}

type ShippingAddress struct {
	Line1      string `json:"line1" validate:"required"`
	City       string `json:"city" validate:"required"`
	PostalCode string `json:"postal_code" validate:"required"`
	Country    string `json:"country" validate:"required,iso3166_1_alpha2"`
}

/** Order Updated Event */

type OrderUpdatedEvent struct {
	BaseOrderEvent
	Data OrderUpdatedData `json:"data" validate:"required"`
}

type OrderUpdatedData struct {
	BaseOrderData
	PreviousStatus OrderStatus `json:"previous_status" validate:"required,OrderStatus"`
	Changes        Changes     `json:"changes" validate:"required"`
}

type Changes struct {
	Status  StatusChange `json:"status" validate:"required"`
	Payment PaymentInfo  `json:"payment" validate:"omitempty"`
}

type StatusChange struct {
	From OrderStatus `json:"from" validate:"required,OrderStatus"`
	To   OrderStatus `json:"to" validate:"required,OrderStatus"`
}

type PaymentInfo struct {
	Method        PaymentMethod `json:"method" validate:"required,PaymentMethod"`
	TransactionID string        `json:"transaction_id" validate:"required"`
	PaidAt        time.Time     `json:"paid_at" validate:"required"`
}

/** Order Shipped Event */

type OrderShippedEvent struct {
	BaseOrderEvent
	Data OrderShippedData `json:"data" validate:"required"`
}

type OrderShippedData struct {
	BaseOrderData
	Shipment Shipment `json:"shipment" validate:"required"`
}

type Shipment struct {
	Carrier           string      `json:"carrier" validate:"required"`
	TrackingNumber    string      `json:"tracking_number" validate:"required"`
	TrackingURL       string      `json:"tracking_url" validate:"required,url"`
	ShippedAt         time.Time   `json:"shipped_at" validate:"required"`
	EstimatedDelivery time.Time   `json:"estimated_delivery" validate:"required,gtfield=ShippedAt"`
	Items             []OrderItem `json:"items" validate:"required,min=1,dive"`
}
