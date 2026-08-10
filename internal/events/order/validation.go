package order

import "github.com/go-playground/validator/v10"

// Validate is a package-level validator wired with custom tags that check
// enum fields against this package's Listed* slices, so the const blocks
// stay the single source of truth for valid enum values.
var Validate = validator.New()

func init() {
	register("EventType", ListedEventTypes)
	register("OrderStatus", ListedOrderStatuses)
	register("PaymentMethod", ListedPaymentMethods)
}

// register wires a validator tag that passes when the field's string value
// matches one of allowed's members.
func register[T ~string](tag string, allowed []T) {
	err := Validate.RegisterValidation(tag, func(fl validator.FieldLevel) bool {
		v := fl.Field().String()
		for _, a := range allowed {
			if string(a) == v {
				return true
			}
		}
		return false
	})
	if err != nil {
		panic(err)
	}
}
