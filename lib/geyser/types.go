package geyser

type SubscribeEvents struct {
	Update func(interface{})
	Error  func(error)
	Eof    func()
}
