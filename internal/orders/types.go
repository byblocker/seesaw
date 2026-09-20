package orders

type OrderType string

const (
	Market OrderType = "MARKET"
	Limit  OrderType = "LIMIT"
)
