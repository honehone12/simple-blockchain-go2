package common

type Result[T interface{}] struct {
	Value T
	Err   error
}
