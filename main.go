package main

import (
	"fmt"

	"github.com/hamza-boudouche/autodev/helpers"
)

func main() {
    kcs, err := helpers.GetK8sClient()
    if err != nil {
        panic(err)
    }
    rc := helpers.CreateRedisClient()
    err = helpers.InitSession(rc, kcs, "hamza-test")
    if err != nil {
        panic(err)
    }

    codeEditorPassword := "supersecret"

    components := []helpers.Component{
        {
            ComponentType: helpers.Code,
            ComponentID: "my-code-editor",
            ComponentMetadata: &helpers.ComponentMetadata{
                Password: &codeEditorPassword,
            },
        },
        {
            ComponentType: helpers.Redis,
            ComponentID: "my-redis",
        },
        {
            ComponentType: helpers.Mongo,
            ComponentID: "my-mongo",
        },
    }

    err = helpers.CreateDeploy(kcs, "hamza-test", components)
    fmt.Println(err)
}
