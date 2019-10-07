package main

import (
	"net/http"

	"github.com/kataras/iris"
)

var formIDs = map[string]string{
	"connect_card_itech":         "0",
	"connect_card_jdd":           "85",
	"growth_track_sign_up_itech": "0",
	"growth_track_sign_up_jdd":   "0",
}

func formResponsesGet(ctx iris.Context) {
	formName := ctx.Params().Get("type")
	modifiedSince := ctx.URLParam("modified_since")

	// if no form name given, return error
	if formName == "" {
		ctx.StatusCode(http.StatusBadRequest)
		ctx.WriteString("Error: No form name provided")
		return
	}

	// build CCB request URL
	urlFormResponses := "https://vouschurch.ccbchurch.com/api.php?srv=form_responses&form_id=" + formIDs[formName]

	if modifiedSince != "" {
		urlFormResponses += "&modified_since=" + modifiedSince
	}

	makeCCBRequest(ctx, urlFormResponses, "GET", formResponseHandler)
}
