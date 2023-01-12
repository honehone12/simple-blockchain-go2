package common

type Result[T interface{}] struct {
	Value T
	Err   error
}

type BlockchainEvent struct {
	Event func()
}
