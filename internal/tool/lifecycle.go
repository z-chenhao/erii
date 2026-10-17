package tool

import "context"

type shutdownSignalKey struct{}

// WithShutdownSignal lets internal coordinators interrupt replay on shutdown
// while ordinary effect executors retain their existing settlement context.
func WithShutdownSignal(ctx context.Context, signal <-chan struct{}) context.Context {
	return context.WithValue(ctx, shutdownSignalKey{}, signal)
}
func ShutdownSignal(ctx context.Context) <-chan struct{} {
	signal, _ := ctx.Value(shutdownSignalKey{}).(<-chan struct{})
	return signal
}
