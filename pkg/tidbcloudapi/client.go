package tidbcloudapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/go-resty/resty/v2"
	"github.com/icholy/digest"
)

var (
	clientInitOnce sync.Once

	restClient *resty.Client
)

func InitClient(publicKey, privateKey string) error {
	clientInitOnce.Do(func() {
		restClient = resty.New()
		restClient.SetTransport(&digest.Transport{
			Username: publicKey,
			Password: privateKey,
		})
	})

	return nil
}

// doRequest wraps resty request, it's a generic method to spawn a HTTP request
func doRequest(method, url string, payload, output interface{}) (*resty.Response, error) {
	request := restClient.R()

	// if payload is not nil, we'll put it on body
	if payload != nil {
		request.SetBody(payload)
	}

	// execute the request
	resp, err := request.Execute(method, url)
	// b, _ := json.Marshal(payload)
	// fmt.Printf("\nRequest: method %s, url %s, response %s\n\n", method, url, resp)
	if err != nil {
		return nil, err
	}

	// if the request return a non-200 response, wrap it with error
	if resp.StatusCode() != http.StatusOK {
		return resp, fmt.Errorf("Failed with status %d and resp %s", resp.StatusCode(), resp)
	}

	// if we need to unmarshal the response into a struct, we pass it here, otherwise pass nil in the argument
	if output != nil {
		return resp, json.Unmarshal(resp.Body(), output)
	}

	return resp, nil
}

func DoGET(url string, payload, output interface{}) (*resty.Response, error) {
	return doRequest(resty.MethodGet, url, payload, output)
}

func DoPOST(url string, payload, output interface{}) (*resty.Response, error) {
	return doRequest(resty.MethodPost, url, payload, output)
}

func doDELETE(url string, payload, output interface{}) (*resty.Response, error) {
	return doRequest(resty.MethodDelete, url, payload, output)
}

func doPATCH(url string, payload, output interface{}) (*resty.Response, error) {
	return doRequest(resty.MethodPatch, url, payload, output)
}
