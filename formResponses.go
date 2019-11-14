package main

import (
	"net/http"
	"encoding/json"
	"time"

	iris "github.com/kataras/iris/v12"
	"github.com/kelseyhightower/envconfig"
	"github.com/mruVOUS/ccb-webflow-api/lib/ccb"
	"github.com/mruVOUS/ccb-webflow-api/lib/vouslog"
	"github.com/sirupsen/logrus"
)

// formNameMap maps the query parameter values to the CCB form IDs
// this is used in building the request to CCB for form responses
var formNameMap = map[string]ccb.FormID{
	"connect_card_itech":         ccb.FormIDConnectCardITech,
	"connect_card_jdd":           ccb.FormIDConnectCardJDD,
	"growth_track_sign_up_itech": ccb.FormIDGrowthTrackSignUpITech,
	"growth_track_sign_up_jdd":   ccb.FormIDGrowthTrackSignUpJDD,
}

// formResponsesGet handles the GET route for form responses.
// it takes a parameter of a form name, and optionally takes a parameter of "modified_since"
// returns form responses in JSON format
func formResponsesGet(ctx iris.Context) {
	logger := vouslog.GetLogger(ctx.Request().Context())

	// Parse query parameters.
	formName := ctx.Params().Get("type")
	modifiedSinceStr := ctx.URLParam("modified_since")

	// Define defaults.
	page := ctx.Params().Get("page")
	if page == "" {
		page = "1"
	}
	pageSize := ctx.Params().Get("pageSize")
	if pageSize == "" {
		pageSize = "10"
	}

	logger.WithFields(logrus.Fields{
		"type":           formName,
		"modified_since": modifiedSinceStr,
		"page": page,
		"page_size": pageSize,
	}).Info("Get form responses.")

	// if no form name given, return error
	if formName == "" {
		ctx.StatusCode(http.StatusBadRequest)
		ctx.WriteString("Error: No form name provided")
		return
	}

	formID, ok := formNameMap[formName]
	if !ok {
		ctx.StatusCode(http.StatusBadRequest)
		ctx.WriteString("Error: Invalid form name.")
		// TODO: Write an error payload about the bad request.
		return
	}

	var modifiedSince *time.Time
	if modifiedSinceStr != "" {
		modTime, err := time.Parse("2006-01-02", modifiedSinceStr)
		if err != nil {
			logger.WithField("modified_since", modifiedSinceStr).Error("Failed to parse modified since.")
			ctx.StatusCode(http.StatusBadRequest)
			ctx.WriteString("Error: Invalid modified since date.")
			// TODO: Write an error payload about the bad request.
			return
		}
		modifiedSince = &modTime
	}

	svcConfig := ccb.Config{}
	envconfig.MustProcess("", &svcConfig)
	svc := ccb.New(svcConfig)
	resp, err := svc.GetFormResponses(ctx.Request().Context(), ccb.GetFormResponsesRequest{
		FormID: formID,
		ModifiedSince: modifiedSince,
		Page: 1,
		PageSize: 10,
		// TODO: Pass in paging parameters.
	})
	if err != nil {
		ctx.StatusCode(http.StatusInternalServerError)
		return
	}

	// TODO: Probably need to implement the next page trick?
	out, err := json.Marshal(resp.Responses)
	if err != nil {
		ctx.StatusCode(http.StatusInternalServerError)
		return
	}

	ctx.StatusCode(http.StatusOK)
	ctx.ContentType("application/json")
	ctx.Write(out)
	return;
}
