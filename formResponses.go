package main

import (
	"net/http"

	"github.com/kataras/iris"
)

// formIDs maps the query parameter values to the CCB form IDs
// this is used in building the request to CCB for form responses
var formIDs = map[string]string{
	"connect_card_itech":         "84",
	"connect_card_jdd":           "85",
	"growth_track_sign_up":       "167",
	"growth_track_sign_up_itech": "70",
	"growth_track_sign_up_jdd":   "69",
}

// formResponsesGet handles the GET route for form responses.
// it takes a parameter of a form name, and optionally takes a parameter of "modified_since"
// returns form responses in JSON format
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
