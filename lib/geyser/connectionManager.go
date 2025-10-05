package geyser

type ConnectionManager struct {
	connections   map[string]*Connection
	subscriptions map[string][]*Subscription
}
