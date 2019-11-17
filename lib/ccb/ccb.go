package ccb

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/mruVOUS/ccb-webflow-api/lib/vouslog"
	"github.com/sirupsen/logrus"
)

// Config holds configuration data needed to communicate with the CCB database.
type Config struct {
	Username       string        `envconfig:"CCB_USERNAME"`
	Password       string        `envconfig:"CCB_PASSWORD"`
	APIURL         string        `envconfig:"CCB_API_URL"`                           // API URL for the CCB API.
	DefaultTimeout time.Duration `envconfig:"CCB_DEFAULT_TIMEOUT"      default:"5s"` // Timeout for HTTP calls to CCB.
}

// FormID represents an enum for a form response in CCB.
type FormID int

// FormID represents enums for types of form responses to retrieve from CCB.
const (
	FormIDConnectCardITech       FormID = 0 // TODO: Set this.
	FormIDConnectCardJDD         FormID = 85
	FormIDGrowthTrackSignUpITech FormID = 2 // TODO: Set this.
	FormIDGrowthTrackSignUpJDD   FormID = 3 // TODO: Set this.
)

// Service defines functions for communicating with the CCB service.
type Service interface {
	// GetFormResponses returns form responses for the supplied form ID.
	GetFormResponses(context.Context, GetFormResponsesRequest) (*GetFormResponsesResponse, error)
}

type defaultService struct {
	config Config
	client *http.Client
}

// New creates a new CCB Service to talk to the Church Community Build (CCB) service.
func New(cfg Config) Service {
	return &defaultService{
		config: cfg,
		client: &http.Client{},
	}
}

const (
	initialBackoffInterval = 100 * time.Millisecond
	maxElapsedTime         = 15 * time.Second
)

// GetFormResponsesRequest represents a request to GetFormResponses.
type GetFormResponsesRequest struct {
	FormID        FormID
	ModifiedSince *time.Time // Day
	Page          int
	PageSize      int
}

// GetFormResponsesResponse represents a response from GetFormResponses.
type GetFormResponsesResponse struct {
	Responses []FormResponse
}

// FormResponse represents the responses to forms such as Connect Cards.
type FormResponse struct {
	ID          string            `json:"id,omitempty"`
	ProfileInfo map[string]string `json:"profile_info,omitempty"`
	Answers     map[string]string `json:"answers,omitempty"`
	Created     string            `json:"created,omitempty"`
	Modified    string            `json:"modified,omitempty"`
}

// GetFormResponses returns form responses for the supplied form ID.
func (svc *defaultService) GetFormResponses(ctx context.Context, req GetFormResponsesRequest) (*GetFormResponsesResponse, error) {
	logger := vouslog.GetLogger(ctx)
	logger.WithFields(logrus.Fields{
		"form_id":   req.FormID,
		"page":      req.Page,
		"page_size": req.PageSize,
	}).Info("Getting form responses from CCB.")

	// Build the query parameters.
	q := url.Values{}
	q.Add("srv", "form_responses")
	q.Add("page", strconv.Itoa(req.Page))
	q.Add("per_page", strconv.Itoa(req.PageSize))
	q.Add("form_id", strconv.Itoa(int(req.FormID)))
	if req.ModifiedSince != nil {
		// Only supports year-month-date.
		q.Add("modified_since", req.ModifiedSince.Format("2006-01-02"))
	}

	// Build the do the HTTP request.
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, svc.config.APIURL+"/api.php?"+q.Encode(), nil)
	if err != nil {
		return nil, errors.New("create request: " + err.Error())
	}
	httpReq.SetBasicAuth(svc.config.Username, svc.config.Password)

	httpResp, err := svc.doRequestWithRetry(ctx, httpReq)
	if err != nil {
		return nil, errors.New("do request with retry: " + err.Error())
	}
	if httpResp.Body != nil {
		defer httpResp.Body.Close()
	}

	if httpResp.StatusCode != http.StatusOK {
		// Log the response here for debugging.
		msg, _ := ioutil.ReadAll(httpResp.Body) // Best effort.
		logger.WithFields(logrus.Fields{
			"ccb_status_code": httpResp.StatusCode,
			"ccb_response":    msg,
		}).Error("Unexpected response from CCB.")
		return nil, errors.New("unexpected response from CCB: " + strconv.Itoa(httpResp.StatusCode))
	}

	formResponses, err := svc.handleFormsCCBResponse(ctx, httpResp)
	if err != nil {
		return nil, errors.New("handle forms in CCB response: " + err.Error())
	}

	return &GetFormResponsesResponse{
		Responses: formResponses,
	}, nil
}

func (svc *defaultService) handleFormsCCBResponse(ctx context.Context, resp *http.Response) ([]FormResponse, error) {
	logger := vouslog.GetLogger(ctx)

	var data ccbResponse
	if err := xml.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, errors.New("unmarshal xml body: " + err.Error())
	}

	// Handle any errors in the form response.
	if data.Response.Errors != nil && len(data.Response.Errors.Error) > 0 {
		// FUTURE: Handle any specific errors needed here coming from CCB in the payload.
		logger.WithField("errors", data.Response.Errors.Error).Error("Error returned from CCB.")
		return nil, errors.New("errors returned from CCB response")
	}

	// Exit if there's no responses. This is fine for empty pages.
	if data.Response.FormResponses == nil || data.Response.FormResponses.Count == 0 {
		return nil, nil
	}

	// Build the FormResponses.
	var formResponses []FormResponse
	for _, v := range data.Response.FormResponses.FormResponse {
		profInfo := map[string]string{} // this will contain profile information
		answers := map[string]string{}  // this will contain the form questions and answers

		// range over profile information and move to a map with info.Name as the key and info.Text as the value
		for _, info := range v.ProfileFields.ProfileInfo {
			profInfo[info.Name] = info.Text
		}

		// range over XML unmarshalled "Answers" and move form questions and answers to a map
		for i, t := range v.Answers.Title {
			answers[t] = v.Answers.Choice[i]
		}

		// fill in the rest of the form data
		f := FormResponse{
			ID:          v.Form.ID,
			ProfileInfo: profInfo,
			Answers:     answers,
			Created:     v.Created,
			Modified:    v.Modified,
		}

		// append the Form Data to formResponses.Responses
		formResponses = append(formResponses, f)
	}
	return formResponses, nil
}

func (svc *defaultService) doRequestWithRetry(ctx context.Context, req *http.Request) (*http.Response, error) {
	if req == nil {
		return nil, errors.New("req is nil")
	}

	expBackoff := backoff.NewExponentialBackOff()
	expBackoff.InitialInterval = initialBackoffInterval
	expBackoff.MaxElapsedTime = maxElapsedTime

	var resp *http.Response
	var retryRespStatusCode int
	var retryCount int
	var retryErr error

	handleRetryError := func(e error) error {
		if e == nil {
			return nil
		}

		retryCount++

		if resp != nil {
			retryRespStatusCode = resp.StatusCode
		}

		retryErr = e
		return retryErr
	}

	logger := vouslog.GetLogger(ctx).WithFields(logrus.Fields{
		"req_url": req.URL.String(),
	})

	if err := backoff.Retry(func() error {
		logger.Data["retry_count"] = retryCount
		if retryCount > 0 {
			logger.Data["retry_error"] = retryErr
			logger.Data["retry_resp_status_code"] = retryRespStatusCode
			if logger.Level >= logrus.DebugLevel {
				logger.Data["retry_resp"] = dumpResponse(resp)
			}
		}
		logger.Info("Calling CCB service.")

		timedCtx, cancel := context.WithTimeout(ctx, svc.config.DefaultTimeout)
		defer cancel() // Prevent a context-leak.
		timedReq := req.WithContext(timedCtx)

		var err error
		if resp, err = svc.client.Do(timedReq); err != nil {
			// No retries on this type of failure coming from invocation.
			logger.WithError(err).Info("Got permanent error from CCB service.")
			return backoff.Permanent(err)
		}

		// Retry on these specific status codes.
		switch resp.StatusCode {
		case http.StatusInternalServerError, http.StatusGatewayTimeout, http.StatusServiceUnavailable:
			return handleRetryError(errors.New("unsuccessful response from CCB service"))
		}

		logger.WithField("resp", dumpResponse(resp)).Debug("CCB service response.")
		return nil
	}, expBackoff); err != nil {
		return nil, fmt.Errorf("failed to call CCB service after %d retries: %s", retryCount, err.Error())
	}

	return resp, nil
}

// // makeCCBRequest performs a request against Church Community Build (CCB).
// func makeCCBRequest(ctx iris.Context, url string, method string) (*CCBResponse, error) {
// 	logger := vouslog.GetLogger(ctx.Request().Context())

// 	// build http request
// 	client := &http.Client{}
// 	timedCtx, cancel := context.WithTimeout(ctx.Request().Context(), 15*time.Second)
// 	defer cancel()
// 	req, err := http.NewRequestWithContext(timedCtx, method, url, nil)
// 	if err != nil {
// 		return nil, errors.New("build request")
// 	}
// 	req.Header.Add("Correlation-Id", ctx.GetHeader("Correlation-Id"))
// 	// req.SetBasicAuth(os.Getenv("CCB_USERNAME"), os.Getenv("CCB_PASSWORD"))
// 	req.SetBasicAuth("123", os.Getenv("CCB_PASSWORD"))

// 	// send request
// 	logger.WithFields(logrus.Fields{
// 		"ccb_method": method,
// 		"ccb_url":    url,
// 	}).Info("Sending request to CCB.")
// 	resp, err := client.Do(req)
// 	if err != nil {
// 		if timedCtx.Err() == context.DeadlineExceeded || timedCtx.Err() == context.Canceled {
// 			return nil, errors.New("timed out")
// 		}
// 		return nil, errors.New("do request: " + err.Error())
// 	}
// 	defer resp.Body.Close()

// 	if logger.Logger.Level == logrus.DebugLevel {
// 		logger.Print("CCB Request:\n" + dumpResponse(resp))
// 	}

// 	if resp.StatusCode != 200 {
// 		// Log the response here for debugging.
// 		msg, _ := ioutil.ReadAll(resp.Body) // Best effort.
// 		logger.WithFields(logrus.Fields{
// 			"ccb_status_code": resp.StatusCode,
// 			"ccb_response":    msg,
// 		}).Error("Unexpected response from CCB.")
// 		return nil, errors.New("unexpected response from CCB: " + strconv.Itoa(resp.StatusCode))
// 	}

// 	var data CCBResponse
// 	if err = xml.NewDecoder(resp.Body).Decode(&data); err != nil {
// 		return nil, errors.New("unmarshal xml body: " + err.Error())
// 	}

// 	return &data, nil
// }

// dumpResponse returns a human readable string representing the request and response
func dumpResponse(resp *http.Response) string {
	var (
		reqBuf, respBuf []byte
		err             error
	)

	if resp.Request != nil {
		if reqBuf, err = httputil.DumpRequestOut(resp.Request, false); err != nil {
			reqBuf = []byte(fmt.Sprintf("[ERROR: %s]", err.Error()))
		}
	}
	if resp != nil {
		if respBuf, err = httputil.DumpResponse(resp, true); err != nil {
			respBuf = []byte(fmt.Sprintf("[ERROR: %s]", err.Error()))
		}
	}
	return fmt.Sprintf("Request\n%s\n\n\nResponse\n%s\n", reqBuf, respBuf)
}

// func whoIsResponseHandler(ctx iris.Context, resp CCBResponse) {
// 	jsonResponse, err := json.Marshal(resp)
// 	if nil != err {
// 		fmt.Println(err) // TODO: this should be logged
// 		ctx.StatusCode(http.StatusInternalServerError)
// 		ctx.WriteString("Error marshalling to JSON")
// 		return
// 	}

// 	// write back the body as JSON
// 	ctx.Write([]byte(jsonResponse))
// }

// formResponseHandler changes the structure of the CCB response data to a more readable structure
// before passing the JSON back to the user.
// func formResponseHandler(ctx iris.Context, resp CCBResponse) {
// 	logger := vouslog.GetLogger(ctx.Request().Context())

// 	// // Handle any errors in the form response.
// 	// if len(resp.Response.Errors.Error) > 0 {
// 	// 	// FUTURE: Handle any specific errors needed here coming from CCB in the payload.
// 	// 	logger.WithField("errors", resp.Response.Errors.Error).Error("Error returned from CCB.")
// 	// 	ctx.StatusCode(http.StatusInternalServerError)
// 	// 	return
// 	// }

// 	// Fill in count field from CCBResponse to a FormResponses struct
// 	// create variables for iterator to fill in.
// 	var formResponses FormResponses
// 	var jsonResponse []byte
// 	var err error

// 	// if there are more than 0 form responses returned, fill in the FormResponses struct
// 	if resp.Response.FormResponses == nil || resp.Response.FormResponses.Count == 0 {
// 		return
// 	}

// 	formResponses.Count = resp.Response.FormResponses.Count

// 	// range over the form responses from CCB
// 	for _, v := range resp.Response.FormResponses.FormResponse {
// 		profInfo := map[string]string{} // this will contain profile information
// 		answers := map[string]string{}  // this will contain the form questions and answers

// 		// range over profile information and move to a map with info.Name as the key and info.Text as the value
// 		for _, info := range v.ProfileFields.ProfileInfo {
// 			profInfo[info.Name] = info.Text
// 		}

// 		// range over XML unmarshalled "Answers" and move form questions and answers to a map
// 		for i, t := range v.Answers.Title {
// 			answers[t] = v.Answers.Choice[i]
// 		}

// 		// fill in the rest of the form data
// 		s := FormData{
// 			ID:          v.Form.ID,
// 			ProfileInfo: profInfo,
// 			Answers:     answers,
// 			Created:     v.Created,
// 			Modified:    v.Modified,
// 		}

// 		// append the Form Data to formResponses.Responses
// 		formResponses.Responses = append(formResponses.Responses, &s)
// 	}
// 	// marshal the formResponses to JSON
// 	jsonResponse, err = json.Marshal(formResponses)

// 	if err != nil {
// 		logger.WithError(err).Error("Failed to marshal response.")
// 		ctx.StatusCode(http.StatusInternalServerError)
// 		return errors.New("marshal response")
// 	}

// 	// write back the body as JSON
// 	ctx.ContentType("application/json")
// 	ctx.Write(jsonResponse)
// }

// CCBResponse represents the xml response from CCB individual search or CCB form response
type ccbResponse struct {
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
			Count      string `xml:"count,attr,omitempty" json:"count,omitempty"`
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
		Errors *struct {
			Error []struct {
				Number  string `xml:"number,attr,omitempty" json:"number,omitempty"`
				Type    string `xml:"type,attr,omitempty" json:"type,omitempty"`
				Message string `xml:",chardata" json:"message,omitempty"`
			} `xml:"error,omitempty" json:"errors,omitempty"`
		} `xml:"errors,omitempty" json:"errors,omitempty"`
	} `xml:"response,omitempty" json:"response,omitempty"`
}
