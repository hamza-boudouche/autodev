package main

import (
	"context"
	"fmt"
	"math/rand"
    "github.com/hamza-boudouche/autodev/pkg/helpers/consistency"
)

var myrandom int = 7

func main() {
    // test saga : get a new random number that's bigger than 5
    // 1- generate new random number between 0 and 9 and store it (as a side effect of this operation) in myrandom variable
    // 2- check if random number is bigger than 5, if not return error
    getRandNumber := func(ctx context.Context) (context.Context, func(context.Context), error) {
        ctx = context.WithValue(ctx, "oldNumber", myrandom)
        myrandom = rand.Intn(10)
        fmt.Println("generated ", myrandom)
        ctx = context.WithValue(ctx, "randomNumber", myrandom)

        cancel := func(ctx context.Context) {
            // reverse the side effects
            fmt.Println("reverting to old value")
            oldValue, ok := ctx.Value("oldNumber").(int)
            if ok {
                myrandom = oldValue
            }
        }
        return ctx, cancel, nil
    }

    checkRandNumber := func(ctx context.Context) (context.Context, func(context.Context), error) {
        if number, ok := ctx.Value("randomNumber").(int); ok {
            if number < 5 {
                fmt.Println("number is smaller")
                return nil, nil, fmt.Errorf("smaller than 5")
            } else {
                return ctx, func(ctx context.Context) {}, nil
            }
        } else {
            return nil, nil, fmt.Errorf("number invalid or not found")
        }
    }

    s := consistency.Saga([]consistency.Transaction{getRandNumber, checkRandNumber})
    s.Run()
    fmt.Println(myrandom)
}

