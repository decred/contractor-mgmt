package client

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"strings"

	"github.com/decred/politeia/util"
	"github.com/gorilla/schema"
	"golang.org/x/net/publicsuffix"

	"github.com/decred/contractor-mgmt/cmswww/api/v1"
	"github.com/decred/contractor-mgmt/cmswww/cmd/cmswwwcli/config"
)

type Ctx struct {
	client            *http.Client
	LastCommandOutput string
}

func NewClient(skipVerify bool) (*Ctx, error) {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: skipVerify,
	}
	tr := &http.Transport{
		TLSClientConfig: tlsConfig,
	}
	jar, err := cookiejar.New(&cookiejar.Options{
		PublicSuffixList: publicsuffix.List,
	})
	if err != nil {
		return nil, err
	}
	return &Ctx{
		client: &http.Client{
			Transport: tr,
			Jar:       jar,
		},
	}, nil
}

func (c *Ctx) Post(route string, requestJSON interface{}, responseJSON interface{}) error {
	return c.makeRequest(http.MethodPost, route, requestJSON, responseJSON)
}

func (c *Ctx) Get(route string, requestJSON interface{}, responseJSON interface{}) error {
	return c.makeRequest(http.MethodGet, route, requestJSON, responseJSON)
}

func (c *Ctx) makeRequest(method, route string, requestJSON interface{}, responseJSON interface{}) error {
	var requestBody []byte
	var queryParams string
	if requestJSON != nil {
		if method == http.MethodGet {
			// GET requests don't have a request body; instead we will populate
			// the query params.
			form := url.Values{}
			err := schema.NewEncoder().Encode(requestJSON, form)
			if err != nil {
				return err
			}

			queryParams = "?" + form.Encode()
		} else {
			var err error
			requestBody, err = json.Marshal(requestJSON)
			if err != nil {
				return err
			}
		}
	}

	var fullRoute string
	if route == v1.RouteRoot {
		fullRoute = config.Host + route
	} else {
		fullRoute = config.Host + v1.APIRoute + route + queryParams
	}

	if config.Verbose {
		log.Printf("Request: %v %v ", method, fullRoute)
		if method != http.MethodGet {
			prettyPrintJSON(requestJSON)
		} else {
			fmt.Println()
		}
	}

	req, err := http.NewRequest(method, fullRoute, bytes.NewReader(requestBody))
	if err != nil {
		return err
	}
	if config.CsrfToken != "" {
		req.Header.Add(v1.CsrfToken, config.CsrfToken)
	}
	r, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		r.Body.Close()
	}()

	if route == v1.RouteRoot {
		// store CSRF and cookies
		c.SetCookies(config.Host, r.Cookies())
		config.SaveCsrf(r.Header.Get(v1.CsrfToken))
	}

	return c.handleResponse(r, responseJSON)
}

func (c *Ctx) handleResponse(r *http.Response, responseJSON interface{}) error {
	responseBody := util.ConvertBodyToByteArray(r.Body, false)

	if config.JSONOutput {
		c.LastCommandOutput = string(responseBody)
		if responseJSON != nil {
			json.Unmarshal(responseBody, responseJSON)
		}
		return nil
	}

	if r.StatusCode != http.StatusOK {
		if config.Verbose {
			log.Printf("Response: %v ", r.Status)
			var errJSON interface{}
			err := json.Unmarshal(responseBody, &errJSON)
			if err == nil {
				prettyPrintJSON(errJSON)
			} else {
				fmt.Println()
			}
		}

		var ue v1.ErrorReply
		err := json.Unmarshal(responseBody, &ue)
		if err != nil {
			return fmt.Errorf("Error: %v", r.Status)
		}

		detailedErr, ok := v1.ErrorStatus[v1.ErrorStatusT(ue.ErrorCode)]
		if ok && ue.ErrorCode != 0 {
			detailedErr += " " + strings.Join(ue.ErrorContext, ", ")
		} else {
			detailedErr = strconv.FormatInt(ue.ErrorCode, 10)
		}

		return fmt.Errorf("%v, %v", r.Status, detailedErr)
	}

	if responseJSON != nil {
		err := json.Unmarshal(responseBody, responseJSON)
		if err != nil {
			return fmt.Errorf("Could not unmarshal reply: %v", err)
		}
	}

	if config.Verbose {
		log.Printf("Response: %v ", r.Status)
		if responseJSON != nil {
			prettyPrintJSON(responseJSON)
		} else {
			fmt.Println()
		}
	}

	return nil
}

func (c *Ctx) Cookies(rawurl string) ([]*http.Cookie, error) {
	u, err := url.Parse(rawurl)
	if err != nil {
		return nil, err
	}
	ck := c.client.Jar.Cookies(u)
	return ck, nil
}

func (c *Ctx) SetCookies(rawurl string, cookies []*http.Cookie) error {
	u, err := url.Parse(rawurl)
	if err != nil {
		return err
	}
	c.client.Jar.SetCookies(u, cookies)
	return nil
}
