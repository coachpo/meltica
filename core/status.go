package core

type OrderStatus string

const (
	OrderNew        OrderStatus = "new"
	OrderPartFilled OrderStatus = "part_filled"
	OrderFilled     OrderStatus = "filled"
	OrderCanceled   OrderStatus = "canceled"
	OrderRejected   OrderStatus = "rejected"
)
