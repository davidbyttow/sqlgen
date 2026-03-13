package sqlgen

import (
	"context"
	"errors"
	"testing"
)

func TestHooksRunOrder(t *testing.T) {
	h := NewHooks()
	var order []int

	h.Add(BeforeInsert, func(ctx context.Context, exec Executor, model any) (context.Context, error) {
		order = append(order, 1)
		return ctx, nil
	})
	h.Add(BeforeInsert, func(ctx context.Context, exec Executor, model any) (context.Context, error) {
		order = append(order, 2)
		return ctx, nil
	})
	h.Add(BeforeInsert, func(ctx context.Context, exec Executor, model any) (context.Context, error) {
		order = append(order, 3)
		return ctx, nil
	})

	_, err := h.Run(context.Background(), nil, BeforeInsert, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(order) != 3 || order[0] != 1 || order[1] != 2 || order[2] != 3 {
		t.Errorf("execution order = %v, want [1 2 3]", order)
	}
}

func TestHooksContextChaining(t *testing.T) {
	type ctxKey string
	h := NewHooks()

	h.Add(BeforeInsert, func(ctx context.Context, exec Executor, model any) (context.Context, error) {
		return context.WithValue(ctx, ctxKey("step"), "first"), nil
	})
	h.Add(BeforeInsert, func(ctx context.Context, exec Executor, model any) (context.Context, error) {
		if ctx.Value(ctxKey("step")) != "first" {
			t.Error("context not chained from first hook")
		}
		return context.WithValue(ctx, ctxKey("step"), "second"), nil
	})

	ctx, err := h.Run(context.Background(), nil, BeforeInsert, nil)
	if err != nil {
		t.Fatal(err)
	}
	if ctx.Value(ctxKey("step")) != "second" {
		t.Error("final context should have second step")
	}
}

func TestHooksErrorStopsExecution(t *testing.T) {
	h := NewHooks()
	ran := false

	h.Add(BeforeInsert, func(ctx context.Context, exec Executor, model any) (context.Context, error) {
		return ctx, errors.New("abort")
	})
	h.Add(BeforeInsert, func(ctx context.Context, exec Executor, model any) (context.Context, error) {
		ran = true
		return ctx, nil
	})

	_, err := h.Run(context.Background(), nil, BeforeInsert, nil)
	if err == nil {
		t.Error("expected error")
	}
	if ran {
		t.Error("second hook should not have run")
	}
}

func TestHooksHas(t *testing.T) {
	h := NewHooks()
	if h.Has(BeforeInsert) {
		t.Error("should not have hooks initially")
	}
	h.Add(BeforeInsert, func(ctx context.Context, exec Executor, model any) (context.Context, error) {
		return ctx, nil
	})
	if !h.Has(BeforeInsert) {
		t.Error("should have hooks after Add")
	}
	if h.Has(AfterInsert) {
		t.Error("should not have AfterInsert hooks")
	}
}

func TestSkipHooks(t *testing.T) {
	h := NewHooks()
	ran := false
	h.Add(BeforeInsert, func(ctx context.Context, exec Executor, model any) (context.Context, error) {
		ran = true
		return ctx, nil
	})

	ctx := SkipHooks(context.Background())
	_, err := h.RunIfEnabled(ctx, nil, BeforeInsert, nil)
	if err != nil {
		t.Fatal(err)
	}
	if ran {
		t.Error("hook should not have run with SkipHooks")
	}
}

func TestRunIfEnabledNormal(t *testing.T) {
	h := NewHooks()
	ran := false
	h.Add(AfterInsert, func(ctx context.Context, exec Executor, model any) (context.Context, error) {
		ran = true
		return ctx, nil
	})

	_, err := h.RunIfEnabled(context.Background(), nil, AfterInsert, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !ran {
		t.Error("hook should have run without SkipHooks")
	}
}

func TestHookReceivesModel(t *testing.T) {
	h := NewHooks()
	type User struct{ Name string }
	var received any

	h.Add(BeforeInsert, func(ctx context.Context, exec Executor, model any) (context.Context, error) {
		received = model
		return ctx, nil
	})

	user := &User{Name: "Alice"}
	_, err := h.Run(context.Background(), nil, BeforeInsert, user)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := received.(*User)
	if !ok {
		t.Fatalf("model type = %T, want *User", received)
	}
	if got.Name != "Alice" {
		t.Errorf("model.Name = %q, want Alice", got.Name)
	}
}

func TestHookCanModifyModel(t *testing.T) {
	h := NewHooks()
	type User struct{ Name string }

	h.Add(BeforeInsert, func(ctx context.Context, exec Executor, model any) (context.Context, error) {
		u := model.(*User)
		u.Name = "Modified"
		return ctx, nil
	})

	user := &User{Name: "Original"}
	_, err := h.Run(context.Background(), nil, BeforeInsert, user)
	if err != nil {
		t.Fatal(err)
	}
	if user.Name != "Modified" {
		t.Errorf("model.Name = %q, want Modified", user.Name)
	}
}
