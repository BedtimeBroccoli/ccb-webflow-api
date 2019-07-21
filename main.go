package main

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/go-kit/kit/endpoint"
	httptransport "github.com/go-kit/kit/transport/http"
)

// CCBService provides operations on strings.
type CCBService interface {
	WhoIs(string) (string, error)
}

// ccbService is a concrete implementation of CCBService
type ccbService struct{}

func (ccbService) WhoIs(name string) (string, error) {
	if name == "" {
		return "No name provided.", ErrEmpty
	}

	urlNameSearch := "https://vouschurch.ccbchurch.com/api.php?srv=individual_search"
	//TODO: If one name is given, search both first and last name and combine responses, currently only searches first name
	//urlToo := ""

	fullName := strings.Fields(name)

	if len(fullName) == 2 {
		urlNameSearch = urlNameSearch + "&first_name=" + fullName[0] + "&last_name=" + fullName[1]
	} else {
		//urlToo = url + "&last_name=" + fullName[0]
		urlNameSearch = urlNameSearch + "&first_name=" + fullName[0]

	}

	var data CCBAPI
	bytexml, err := xml.Marshal(&data)

	client := &http.Client{}
	req, err := http.NewRequest("POST", urlNameSearch, bytes.NewBuffer(bytexml))
	if err != nil {
		fmt.Println(err)
	}
	req.SetBasicAuth(os.Getenv("USERNAME"), os.Getenv("PASSWORD"))
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
	}
	defer resp.Body.Close()

	respBody, _ := ioutil.ReadAll(resp.Body)
	err = xml.Unmarshal([]byte(respBody), &data)
	if err != nil {
		fmt.Println(err)
	}
	jsonResponse, err := json.Marshal(data)
	if nil != err {
		fmt.Println("Error marshalling to JSON", err)
	}
	//fmt.Println(string(jsonResponse))
	log.Println(respBody)
	return string(jsonResponse), nil
}

// ErrEmpty is returned when an input string is empty.
var ErrEmpty = errors.New("empty string")

// Endpoints are a primary abstraction in go-kit. An endpoint represents a single RPC (method in our service interface)
func makeWhoIsEndpoint(svc CCBService) endpoint.Endpoint {
	return func(_ context.Context, request interface{}) (interface{}, error) {
		req := request.(CCBPersonRequest)
		v, err := svc.WhoIs(req.Name)
		if err != nil {
			return CCBPersonResponse{"", err.Error()}, nil
		}
		return CCBPersonResponse{v, ""}, nil
	}
}

// Transports expose the service to the network. Create endpoints for slack app
func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	svc := ccbService{}

	WhoIsHandler := httptransport.NewServer(
		makeWhoIsEndpoint(svc),
		decodeWhoIsRequest,
		encodeResponse,
	)

	http.Handle("/WhoIs", WhoIsHandler)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func decodeWhoIsRequest(_ context.Context, r *http.Request) (interface{}, error) {
	var request CCBPersonRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		return nil, err
	}
	return request, nil
}

func encodeResponse(_ context.Context, w http.ResponseWriter, response interface{}) error {
	return json.NewEncoder(w).Encode(response)
}

// CCBPersonRequest Struct is used for the incoming /whois Person POST from Slack
type CCBPersonRequest struct {
	Name string `json:"name,omitempty"`
}

// CCBPersonResponse Struct is used for the outgoing /whois Slack request
type CCBPersonResponse struct {
	Name  string `json:"name,omitempty"`
	Error string `json:"error,omitempty"`
}

//CCBAPI represents the xml response from CCB individual search
type CCBAPI struct {
	XMLName xml.Name `xml:"ccb_api,omitempty"`
	Request struct {
		Parameters struct {
			Argument []struct {
				Value string `xml:"value,attr,omitempty"`
				Name  string `xml:"name,attr,omitempty"`
			} `xml:"argument,omitempty"`
		} `xml:"parameters,omitempty"`
	} `xml:"request,omitempty"`
	Response struct {
		Service       string `xml:"service,omitempty"`
		ServiceAction string `xml:"service_action,omitempty"`
		Availability  string `xml:"availability,omitempty"`
		Individuals   struct {
			Count      string `xml:"count,attr,omitempty"`
			Individual struct {
				ID           string `xml:"id,attr,omitempty"`
				SyncID       string `xml:"sync_id,omitempty"`
				OtherID      string `xml:"other_id,omitempty"`
				GivingNumber string `xml:"giving_number,omitempty"`
				Campus       struct {
					ID string `xml:"id,attr,omitempty"`
				} `xml:"campus,omitempty"`
				Family struct {
					ID string `xml:"id,attr,omitempty"`
				} `xml:"family,omitempty"`
				FamilyImage          string `xml:"family_image,omitempty"`
				FamilyPosition       string `xml:"family_position,omitempty"`
				FamilyMembers        string `xml:"family_members,omitempty"`
				FirstName            string `xml:"first_name,omitempty"`
				LastName             string `xml:"last_name,omitempty"`
				MiddleName           string `xml:"middle_name,omitempty"`
				LegalFirstName       string `xml:"legal_first_name,omitempty"`
				FullName             string `xml:"full_name,omitempty"`
				Salutation           string `xml:"salutation,omitempty"`
				Suffix               string `xml:"suffix,omitempty"`
				Image                string `xml:"image,omitempty"`
				Email                string `xml:"email,omitempty"`
				Allergies            string `xml:"allergies,omitempty"`
				ConfirmedNoAllergies string `xml:"confirmed_no_allergies,omitempty"`
				Addresses            struct {
					Address []struct {
						Type          string `xml:"type,attr,omitempty"`
						StreetAddress string `xml:"street_address,omitempty"`
						City          string `xml:"city,omitempty"`
						State         string `xml:"state,omitempty"`
						Zip           string `xml:"zip,omitempty"`
						Country       struct {
							Code string `xml:"code,attr,omitempty"`
						} `xml:"country,omitempty"`
						Line1     string `xml:"line_1,omitempty"`
						Line2     string `xml:"line_2,omitempty"`
						Latitude  string `xml:"latitude,omitempty"`
						Longitude string `xml:"longitude,omitempty"`
					} `xml:"address,omitempty"`
				} `xml:"addresses,omitempty"`
				Phones struct {
					Phone []struct {
						Type string `xml:"type,attr,omitempty"`
					} `xml:"phone,omitempty"`
				} `xml:"phones,omitempty"`
				MobileCarrier struct {
					ID string `xml:"id,attr,omitempty"`
				} `xml:"mobile_carrier,omitempty"`
				Gender         string `xml:"gender,omitempty"`
				MaritalStatus  string `xml:"marital_status,omitempty"`
				Birthday       string `xml:"birthday,omitempty"`
				Anniversary    string `xml:"anniversary,omitempty"`
				Baptized       string `xml:"baptized,omitempty"`
				Deceased       string `xml:"deceased,omitempty"`
				MembershipType struct {
					ID string `xml:"id,attr,omitempty"`
				} `xml:"membership_type,omitempty"`
				MembershipDate          string `xml:"membership_date,omitempty"`
				MembershipEnd           string `xml:"membership_end,omitempty"`
				ReceiveEmailFromChurch  string `xml:"receive_email_from_church,omitempty"`
				DefaultNewGroupMessages string `xml:"default_new_group_messages,omitempty"`
				DefaultNewGroupComments string `xml:"default_new_group_comments,omitempty"`
				DefaultNewGroupDigest   string `xml:"default_new_group_digest,omitempty"`
				DefaultNewGroupSms      string `xml:"default_new_group_sms,omitempty"`
				PrivacySettings         struct {
					ProfileListed  string `xml:"profile_listed,omitempty"`
					MailingAddress struct {
						ID string `xml:"id,attr,omitempty"`
					} `xml:"mailing_address,omitempty"`
					HomeAddress struct {
						ID string `xml:"id,attr,omitempty"`
					} `xml:"home_address,omitempty"`
					HomePhone struct {
						ID string `xml:"id,attr,omitempty"`
					} `xml:"home_phone,omitempty"`
					WorkPhone struct {
						ID string `xml:"id,attr,omitempty"`
					} `xml:"work_phone,omitempty"`
					MobilePhone struct {
						ID string `xml:"id,attr,omitempty"`
					} `xml:"mobile_phone,omitempty"`
					EmergencyPhone struct {
						ID string `xml:"id,attr,omitempty"`
					} `xml:"emergency_phone,omitempty"`
					Birthday struct {
						ID string `xml:"id,attr,omitempty"`
					} `xml:"birthday,omitempty"`
					Anniversary struct {
						ID string `xml:"id,attr,omitempty"`
					} `xml:"anniversary,omitempty"`
					Gender struct {
						ID string `xml:"id,attr,omitempty"`
					} `xml:"gender,omitempty"`
					MaritalStatus struct {
						ID string `xml:"id,attr,omitempty"`
					} `xml:"marital_status,omitempty"`
					UserDefinedFields struct {
						ID string `xml:"id,attr,omitempty"`
					} `xml:"user_defined_fields,omitempty"`
					Allergies struct {
						ID string `xml:"id,attr,omitempty"`
					} `xml:"allergies,omitempty"`
				} `xml:"privacy_settings,omitempty"`
				Active  string `xml:"active,omitempty"`
				Creator struct {
					ID string `xml:"id,attr,omitempty"`
				} `xml:"creator,omitempty"`
				Modifier struct {
					ID string `xml:"id,attr,omitempty"`
				} `xml:"modifier,omitempty"`
				Created                   string `xml:"created,omitempty"`
				Modified                  string `xml:"modified,omitempty"`
				UserDefinedTextFields     string `xml:"user_defined_text_fields,omitempty"`
				UserDefinedDateFields     string `xml:"user_defined_date_fields,omitempty"`
				UserDefinedPulldownFields string `xml:"user_defined_pulldown_fields,omitempty"`
			} `xml:"individual,omitempty"`
		} `xml:"individuals,omitempty"`
	} `xml:"response,omitempty"`
}
