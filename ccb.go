package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/kataras/iris"
)

// responseHandler is a function that handles the response from CCB.
// depending on the response from CCB, we may want to reformat the structs before passing them back to the user.
type responseHandler func(ctx iris.Context, response CCBResponse)

// makeCCBRequest can currently handle
func makeCCBRequest(ctx iris.Context, url string, method string, handler responseHandler) {
	// build http request
	client := &http.Client{}
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		fmt.Println(err) // TODO: this should be logged
		ctx.StatusCode(http.StatusInternalServerError)
		ctx.WriteString("Error: Unable to build request to database. " + err.Error())
		return
	}
	req.SetBasicAuth(os.Getenv("CCB_USERNAME"), os.Getenv("CCB_PASSWORD"))

	// send request
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println(err) // TODO: this should be logged
		ctx.StatusCode(http.StatusInternalServerError)
		ctx.WriteString("Error: Unable to send request to database. " + err.Error())
		return
	}

	// TODO: Error handling for error responses from CCB
	defer resp.Body.Close()

	var data CCBResponse

	respBody, _ := ioutil.ReadAll(resp.Body)
	err = xml.Unmarshal(respBody, &data)
	if err != nil {
		fmt.Println(err) // TODO: this should be logged
		ctx.StatusCode(http.StatusInternalServerError)
		ctx.WriteString("Error unmarshalling response from database")
		return
	}

	handler(ctx, data)
}

func whoIsResponseHandler(ctx iris.Context, resp CCBResponse) {
	jsonResponse, err := json.Marshal(resp)
	if nil != err {
		fmt.Println(err) // TODO: this should be logged
		ctx.StatusCode(http.StatusInternalServerError)
		ctx.WriteString("Error marshalling to JSON")
		return
	}

	// write back the body as JSON
	ctx.Write([]byte(jsonResponse))
}

// formResponseHandler changes the structure of the CCB response data to a more readable structure
// before passing the JSON back to the user.
func formResponseHandler(ctx iris.Context, resp CCBResponse) {
	// fill in count field from CCBResponse to a FormResponses struct
	// create variables for iterator to fill in.
	var formResponses FormResponses
	formResponses.Count = resp.Response.FormResponses.Count
	var jsonResponse []byte
	var err error

	// if there are more than 0 form responses returned, fill in the FormResponses struct
	if formResponses.Count != 0 {
		// range over the form responses from CCB
		for _, v := range resp.Response.FormResponses.FormResponse {
			profInfo := map[string]string{} // this will contain profile information
			answers := map[string]string{}  // this will contain the form questions and answers

			// range over profile information and move to a map with info.Name as the key and info.Text as the value
			for _, info := range v.ProfileFields.ProfileInfo {
				profInfo[info.Name] = info.Text
			}

			// range over XML unmarshalled "Answers" and move form questions and answers to a map
			for i, questionTitle := range v.Answers.Title {
				answers[questionTitle] = v.Answers.Choice[i]
			}

			// fill in the rest of the form data
			formData := FormData{
				ID:          v.Form.ID,
				ProfileInfo: profInfo,
				Answers:     answers,
				Created:     v.Created,
				Modified:    v.Modified,
			}

			// append the Form Data to formResponses.Responses
			formResponses.Responses = append(formResponses.Responses, &formData)
		}
		// marshal the formResponses to JSON
		jsonResponse, err = json.Marshal(formResponses)
	} else {
		// if there are no results, return a not found Status code and message
		fmt.Println(err) // TODO: this should be logged
		ctx.StatusCode(http.StatusNotFound)
		ctx.WriteString("No results found")
		return
	}

	if err != nil {
		fmt.Println(err) // TODO: this should be logged
		ctx.StatusCode(http.StatusInternalServerError)
		ctx.WriteString("Error marshalling to JSON")
		return
	}

	// write back the body as JSON
	ctx.Write([]byte(jsonResponse))
}

//CCBAPI represents the xml response from CCB individual search or CCB form response
type CCBResponse struct {
	Request struct {
		Parameters struct {
			Argument []struct {
				Value string `xml:"value,attr,omitempty" json:"value,omitempty"`
				Name  string `xml:"name,attr,omitempty" json:"name,omitempty"`
			} `xml:"argument,omitempty" json:"argument,omitempty"`
		} `xml:"parameters,omitempty" json:"parameters,omitempty"`
	} `xml:"request,omitempty" json:"value,omitempty"`
	Response struct {
		Service       string `xml:"service,omitempty" json:"service,omitempty"`
		ServiceAction string `xml:"service_action,omitempty" json:"service_action,omitempty"`
		Availability  string `xml:"availability,omitempty" json:"availability,omitempty"`
		Individuals   *struct {
			Count      int `xml:"count,attr,omitempty" json:"count,omitempty"`
			Individual *struct {
				ID           string `xml:"id,attr,omitempty" json:"id,omitempty"`
				GivingNumber string `xml:"giving_number,omitempty" json:"giving_number,omitempty"`
				Campus       *struct {
					ID string `xml:"id,attr,omitempty" json:"id,omitempty"`
				} `xml:"campus,omitempty" json:"campus,omitempty"`
				Family *struct {
					ID string `xml:"id,attr,omitempty" json:"id,omitempty"`
				} `xml:"family,omitempty" json:"family,omitempty"`
				FamilyImage          string `xml:"family_image,omitempty" json:"family_image,omitempty"`
				FamilyPosition       string `xml:"family_position,omitempty" json:"family_position,omitempty"`
				FamilyMembers        string `xml:"family_members,omitempty" json:"family_members,omitempty"`
				FirstName            string `xml:"first_name,omitempty" json:"first_name,omitempty"`
				LastName             string `xml:"last_name,omitempty" json:"last_name,omitempty"`
				MiddleName           string `xml:"middle_name,omitempty" json:"middle_name,omitempty"`
				LegalFirstName       string `xml:"legal_first_name,omitempty" json:"legal_first_name,omitempty"`
				FullName             string `xml:"full_name,omitempty" json:"full_name,omitempty"`
				Salutation           string `xml:"salutation,omitempty" json:"salutation,omitempty"`
				Suffix               string `xml:"suffix,omitempty" json:"suffix,omitempty"`
				Image                string `xml:"image,omitempty" json:"image,omitempty"`
				Email                string `xml:"email,omitempty" json:"email,omitempty"`
				Allergies            string `xml:"allergies,omitempty" json:"allergies,omitempty"`
				ConfirmedNoAllergies string `xml:"confirmed_no_allergies,omitempty" json:"confirmed_no_allergies,omitempty"`
				Addresses            *struct {
					Address []*struct {
						Type          string `xml:"type,attr,omitempty" json:"type,attr,omitempty"`
						StreetAddress string `xml:"street_address,omitempty" json:"street_address,omitempty"`
						City          string `xml:"city,omitempty" json:"city,omitempty"`
						State         string `xml:"state,omitempty" json:"state,omitempty"`
						Zip           string `xml:"zip,omitempty" json:"zip,omitempty"`
						Country       *struct {
							Code string `xml:"code,attr,omitempty" json:"code,omitempty"`
						} `xml:"country,omitempty" json:"country,omitempty"`
						Line1     string `xml:"line_1,omitempty" json:"line_1,omitempty"`
						Line2     string `xml:"line_2,omitempty" json:"line_2,omitempty"`
						Latitude  string `xml:"latitude,omitempty" json:"latitude,omitempty"`
						Longitude string `xml:"longitude,omitempty" json:"longitude,omitempty"`
					} `xml:"address,omitempty" json:"address,omitempty"`
				} `xml:"addresses,omitempty" json:"addresses,omitempty"`
				Phones []*struct {
					Type   string `xml:"type,attr,omitempty" json:"type,omitempty"`
					Number string `xml:",chardata" json:"number,omitempty"`
				} `xml:"phones>phone,omitempty" json:"phones,omitempty"`
				MobileCarrier *struct {
					ID string `xml:"id,attr,omitempty" json:"id,omitempty"`
				} `xml:"mobile_carrier,omitempty" json:"mobile_carrier,omitempty"`
				Gender         string `xml:"gender,omitempty" json:"gender,omitempty"`
				MaritalStatus  string `xml:"marital_status,omitempty" json:"marital_status,omitempty"`
				Birthday       string `xml:"birthday,omitempty" json:"birthday,omitempty"`
				Anniversary    string `xml:"anniversary,omitempty" json:"anniversary,omitempty"`
				Baptized       string `xml:"baptized,omitempty" json:"baptized,omitempty"`
				Deceased       string `xml:"deceased,omitempty" json:"deceased,omitempty"`
				MembershipType *struct {
					ID string `xml:"id,attr,omitempty" json:"id,omitempty"`
				} `xml:"membership_type,omitempty" json:"membership_type,omitempty"`
				MembershipDate          string `xml:"membership_date,omitempty" json:"membership_date,omitempty"`
				MembershipEnd           string `xml:"membership_end,omitempty" json:"membership_end,omitempty"`
				ReceiveEmailFromChurch  string `xml:"receive_email_from_church,omitempty" json:"receive_email_from_church,omitempty"`
				DefaultNewGroupMessages string `xml:"default_new_group_messages,omitempty" json:"default_new_group_messages,omitempty"`
				DefaultNewGroupComments string `xml:"default_new_group_comments,omitempty" json:"default_new_group_comments,omitempty"`
				DefaultNewGroupDigest   string `xml:"default_new_group_digest,omitempty" json:"default_new_group_digest,omitempty"`
				DefaultNewGroupSms      string `xml:"default_new_group_sms,omitempty" json:"default_new_group_sms,omitempty"`
				PrivacySettings         *struct {
					ProfileListed  string `xml:"profile_listed,omitempty" json:"profile_listed,omitempty"`
					MailingAddress *struct {
						ID string `xml:"id,attr,omitempty" json:"id,omitempty"`
					} `xml:"mailing_address,omitempty" json:"mailing_address,omitempty"`
					HomeAddress *struct {
						ID string `xml:"id,attr,omitempty" json:"id,omitempty"`
					} `xml:"home_address,omitempty" json:"home_address,omitempty"`
					HomePhone *struct {
						ID string `xml:"id,attr,omitempty" json:"id,omitempty"`
					} `xml:"home_phone,omitempty" json:"home_phone,omitempty"`
					WorkPhone *struct {
						ID string `xml:"id,attr,omitempty" json:"id,omitempty"`
					} `xml:"work_phone,omitempty" json:"work_phone,omitempty"`
					MobilePhone *struct {
						ID string `xml:"id,attr,omitempty" json:"id,omitempty"`
					} `xml:"mobile_phone,omitempty" json:"mobile_phone,omitempty"`
					EmergencyPhone *struct {
						ID string `xml:"id,attr,omitempty" json:"id,omitempty"`
					} `xml:"emergency_phone,omitempty" json:"emergency_phone,omitempty"`
					Birthday *struct {
						ID string `xml:"id,attr,omitempty" json:"id,omitempty"`
					} `xml:"birthday,omitempty" json:"birthday,omitempty"`
					Anniversary *struct {
						ID string `xml:"id,attr,omitempty" json:"id,omitempty"`
					} `xml:"anniversary,omitempty" json:"anniversary,omitempty"`
					Gender *struct {
						ID string `xml:"id,attr,omitempty" json:"id,omitempty"`
					} `xml:"gender,omitempty" json:"gender,omitempty"`
					MaritalStatus *struct {
						ID string `xml:"id,attr,omitempty" json:"id,omitempty"`
					} `xml:"marital_status,omitempty" json:"marital_status,omitempty"`
					UserDefinedFields *struct {
						ID string `xml:"id,attr,omitempty" json:"id,omitempty"`
					} `xml:"user_defined_fields,omitempty" json:"user_defined_fields,omitempty"`
					Allergies *struct {
						ID string `xml:"id,attr,omitempty" json:"id,omitempty"`
					} `xml:"allergies,omitempty" json:"allergies,omitempty"`
				} `xml:"privacy_settings,omitempty" json:"privacy_settings,omitempty"`
				Active  string `xml:"active,omitempty" json:"active,omitempty"`
				Creator *struct {
					ID string `xml:"id,attr,omitempty" json:"id,omitempty"`
				} `xml:"creator,omitempty" json:"creator,omitempty"`
				Modifier *struct {
					ID string `xml:"id,attr,omitempty" json:"id,omitempty"`
				} `xml:"modifier,omitempty" json:"modifier,omitempty"`
				Created                   string `xml:"created,omitempty" json:"created,omitempty"`
				Modified                  string `xml:"modified,omitempty" json:"modified,omitempty"`
				UserDefinedTextFields     string `xml:"user_defined_text_fields,omitempty" json:"user_defined_text_fields,omitempty"`
				UserDefinedDateFields     string `xml:"user_defined_date_fields,omitempty" json:"user_defined_date_fields,omitempty"`
				UserDefinedPulldownFields string `xml:"user_defined_pulldown_fields,omitempty" json:"user_defined_pulldown_fields,omitempty"`
			} `xml:"individual,omitempty" json:"individual,omitempty"`
		} `xml:"individuals,omitempty" json:"individuals,omitempty"`
		FormResponses *struct {
			Count        int `xml:"count,attr,omitempty" json:"count,omitempty"`
			FormResponse []*struct {
				ID   string `xml:"id,attr,omitempty" json:"id,omitempty"`
				Form *struct {
					ID string `xml:"id,attr" json:"id,omitempty"`
				} `xml:"form,omitempty" json:"form,omitempty"`
				Individual *struct {
					Name string `xml:",chardata" json:"name,omitempty"`
					ID   string `xml:"id,attr,omitempty" json:"id,omitempty"`
				} `xml:"individual,omitempty" json:"individual,omitempty"`
				Created       string `xml:"created,omitempty" json:"created,omitempty"`
				Modified      string `xml:"modified,omitempty" json:"modified,omitempty"`
				ProfileFields *struct {
					ProfileInfo []*struct {
						Name string `xml:"name,attr,omitempty" json:"name,omitempty"`
						Text string `xml:",chardata" json:"text,omitempty"`
					} `xml:"profile_info,omitempty" json:"profile_info,omitempty"`
				} `xml:"profile_fields,omitempty" json:"profile_fields,omitempty"`
				Answers *struct {
					Title  []string `xml:"title,omitempty" json:"title,omitempty"`
					Choice []string `xml:"choice,omitempty" json:"choice,omitempty"`
				} `xml:"answers,omitempty" json:"answers,omitempty"`
				PaymentInfo string `xml:"payment_info,omitempty"  json:"payment_info,omitempty"`
			} `xml:"form_response,omitempty" json:"form_response,omitempty"`
		} `xml:"form_responses,omitempty" json:"form_responses,omitempty"`
	} `xml:"response,omitempty" json:"response,omitempty"`
}

type FormResponses struct {
	Count     int         `json:"count"`
	Responses []*FormData `json:"responses"`
}

type FormData struct {
	ID          string            `json:"id"`
	ProfileInfo map[string]string `json:"profile_info"`
	Answers     map[string]string `json:"answers"`
	Created     string            `json:"created"`
	Modified    string            `json:"modified"`
}
