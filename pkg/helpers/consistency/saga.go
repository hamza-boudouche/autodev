package consistency

import "context"

type Saga []Transaction

func (s Saga) Run() (context.Context, error) {
    cancels := make([]func(context.Context), 0, len(s))
    ctx := context.Background()
    for _, transaction := range s {
        newCtx, cancel, err := transaction(ctx)
        if err != nil {
            for i := len(cancels) - 1; i >= 0; i-- {
                cancels[i](ctx)
            }
            return ctx, err
        }
        cancels = append(cancels, cancel)
        ctx = newCtx
    }
    return ctx, nil
}

type Transaction func(context.Context) (context.Context, func(context.Context), error)
