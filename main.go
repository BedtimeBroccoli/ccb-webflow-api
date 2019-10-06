package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/kataras/iris"
	"github.com/kataras/iris/middleware/basicauth"
)

func init() {

}

func main() {
	app := iris.New()

	fmt.Println(os.Getenv("GO_API_USERNAME"))

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
	needAuth.Get("/form_responses/{type: string}", func(ctx iris.Context) {})

	// start API
	app.Run(iris.Addr(":8080"))
}

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

	// build http request
	client := &http.Client{}
	req, err := http.NewRequest("GET", urlNameSearch, nil)
	if err != nil {
		fmt.Println(err)
		// TODO: better error handling here
		return
	}
	req.SetBasicAuth(os.Getenv("USERNAME"), os.Getenv("PASSWORD"))

	// send request
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		// TODO: better error handling here
		return
	}
	defer resp.Body.Close()

	// marshal body to XML, then JSON
	var data CCBAPI
	respBody, _ := ioutil.ReadAll(resp.Body)
	err = xml.Unmarshal(respBody, &data)
	if err != nil {
		fmt.Println(err)
		// TODO: better error handling here
		return
	}
	jsonResponse, err := json.Marshal(data)
	if nil != err {
		ctx.StatusCode(http.StatusInternalServerError)
		ctx.WriteString("Error marshalling to JSON")
		return
	}

	// write back the body as JSON
	ctx.Write([]byte(jsonResponse))
}
