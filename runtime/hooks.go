package runtime

import "context"

// Hook is a function that runs before or after a database operation.
// It receives a context and returns a (possibly modified) context and an error.
// Returning an error from a "before" hook cancels the operation.
type Hook func(ctx context.Context) (context.Context, error)

// HookPoint identifies when a hook fires.
type HookPoint int

const (
	BeforeInsert HookPoint = iota
	AfterInsert
	BeforeUpdate
	AfterUpdate
	BeforeDelete
	AfterDelete
	BeforeUpsert
	AfterUpsert
	AfterSelect
)

// Hooks stores registered hooks per hook point.
type Hooks struct {
	hooks map[HookPoint][]Hook
}

// NewHooks creates an empty Hooks registry.
func NewHooks() *Hooks {
	return &Hooks{hooks: make(map[HookPoint][]Hook)}
}

// Add registers a hook at the given point.
func (h *Hooks) Add(point HookPoint, fn Hook) {
	h.hooks[point] = append(h.hooks[point], fn)
}

// Run executes all hooks registered at the given point in order.
// The context is chained through each hook. If any hook returns an error,
// execution stops and the error is returned.
func (h *Hooks) Run(ctx context.Context, point HookPoint) (context.Context, error) {
	fns := h.hooks[point]
	var err error
	for _, fn := range fns {
		ctx, err = fn(ctx)
		if err != nil {
			return ctx, err
		}
	}
	return ctx, nil
}

// Has returns true if any hooks are registered at the given point.
func (h *Hooks) Has(point HookPoint) bool {
	return len(h.hooks[point]) > 0
}

// hooksKey is the context key for skipping hooks.
type hooksKey struct{}

// SkipHooks returns a context that tells hook execution to skip.
func SkipHooks(ctx context.Context) context.Context {
	return context.WithValue(ctx, hooksKey{}, true)
}

// ShouldSkipHooks returns true if SkipHooks was called on this context.
func ShouldSkipHooks(ctx context.Context) bool {
	v, _ := ctx.Value(hooksKey{}).(bool)
	return v
}

// RunIfEnabled runs hooks only if SkipHooks wasn't called on the context.
func (h *Hooks) RunIfEnabled(ctx context.Context, point HookPoint) (context.Context, error) {
	if ShouldSkipHooks(ctx) {
		return ctx, nil
	}
	return h.Run(ctx, point)
}
