package tracer

type Tracer interface {
	Go(name string, f func())
}
