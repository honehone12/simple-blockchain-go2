package common

type Result[T interface{}] struct {
	Value T
	Err   error
}

type BlockchainEvent[T interface{}] struct {
	Event func(t T) error
}
