package main

import (
	"fmt"

	"github.com/kataras/iris"
)

var formIDs = map[string]int{
	"connect_card_itech":         0,
	"connect_card_jdd":           0,
	"growth_track_sign_up_itech": 0,
	"growth_track_sign_up_jdd":   0,
}

func formResponsesGet(ctx iris.Context) {
	formName := ctx.Params().Get("type")
	fmt.Println(formName)
}
