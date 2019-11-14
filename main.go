package main

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	iris "github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/middleware/basicauth"
	"github.com/mruVOUS/ccb-webflow-api/lib/middleware"
	"github.com/sirupsen/logrus"
	prefixed "github.com/x-cray/logrus-prefixed-formatter"
)

func main() {
	setupLogging()

	app := iris.New()

	// Recover middleware recovers from any panics and writes a 500 if there was one.
	// app.Use(recover.New())

	app.Use(middleware.NewLogging())

	// basic auth set up
	authConfig := basicauth.Config{
		Users:   map[string]string{os.Getenv("GO_API_USERNAME"): os.Getenv("GO_API_PASSWORD")},
		Realm:   "Authorization Required", // defaults to "Authorization Required"
		Expires: time.Duration(30) * time.Minute,
	}
	authentication := basicauth.New(authConfig)

	// redirect all requests to authenticated routes
	app.Get("/", func(ctx iris.Context) { ctx.Redirect("/admin") })

	// set up authenticated routes
	needAuth := app.Party("/admin", authentication)
	needAuth.Get("/whois", getPerson)
	needAuth.Get("/form_responses/{type: string}", formResponsesGet)

	portNum := 8080
	logrus.WithField("port", portNum).Info("Starting server.")

	// start API
	app.Run(iris.Addr(":" + strconv.Itoa(portNum)))
}

// TODO: Move this into a separate file.
func getPerson(ctx iris.Context) {
	// get name param from URL
	name := ctx.URLParam("name")

	// if no name given, return error
	if name == "" {
		ctx.StatusCode(http.StatusBadRequest)
		ctx.WriteString("Error: No name provided")
		return
	}

	// build CCB request URL
	urlNameSearch := "https://vouschurch.ccbchurch.com/api.php?srv=individual_search"

	fullName := strings.Fields(name)
	if len(fullName) == 2 {
		urlNameSearch = urlNameSearch + "&first_name=" + fullName[0] + "&last_name=" + fullName[1]
	} else {
		urlNameSearch = urlNameSearch + "&first_name=" + fullName[0]
	}

	// makeCCBRequest(ctx, urlNameSearch, "GET", whoIsResponseHandler)
}

// setupLogging sets up test logging to respect the LOG_LEVEL env var and defaults
// to plain text with colors output.
func setupLogging() {
	// Default to error. Allow override using LOG_LEVEL env var.
	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel != "" {
		lvl, err := logrus.ParseLevel(logLevel)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid value for LOG_LEVEL env var: "+logLevel)
			os.Exit(1)
		}
		logrus.SetLevel(lvl)
	} else {
		logrus.SetLevel(logrus.InfoLevel)
	}

	logType := os.Getenv("LOG_TYPE")
	if logType == "json" {
		logrus.SetFormatter(&logrus.JSONFormatter{})
	} else {
		logrus.SetFormatter(&prefixed.TextFormatter{
			DisableColors:    false,
			ForceColors:      true,
			ForceFormatting:  true,
			DisableTimestamp: true,
		})
	}

	// Set so the library methods that init their own logging follow the same log level.
	os.Setenv("LOG_LEVEL", logrus.GetLevel().String())
}
