package worker

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"

	"github.com/google/uuid"

	"github.com/osbuild/osbuild-composer/internal/worker/api"
)

type Client struct {
	server    *url.URL
	requester *http.Client
}

type Job interface {
	Id() uuid.UUID
	Type() string
	Args(args interface{}) error
	DynamicArgs(i int, args interface{}) error
	NDynamicArgs() int
	Update(result interface{}) error
	Canceled() (bool, error)
	UploadArtifact(name string, reader io.Reader) error
}

type job struct {
	requester        *http.Client
	id               uuid.UUID
	location         string
	artifactLocation string
	jobType          string
	args             json.RawMessage
	dynamicArgs      []json.RawMessage
}

func NewClient(baseURL string, conf *tls.Config) (*Client, error) {
	server, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}

	server, err = server.Parse(api.BasePath + "/")
	if err != nil {
		panic(err)
	}

	requester := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: conf,
		},
	}

	return &Client{server, requester}, nil
}

func NewClientUnix(path string) *Client {
	server, err := url.Parse("http://localhost/")
	if err != nil {
		panic(err)
	}

	server, err = server.Parse(api.BasePath + "/")
	if err != nil {
		panic(err)
	}

	requester := &http.Client{
		Transport: &http.Transport{
			DialContext: func(context context.Context, network, addr string) (net.Conn, error) {
				return net.Dial("unix", path)
			},
		},
	}

	return &Client{server, requester}
}

func (c *Client) RequestJob(types []string, owners []string) (Job, error) {
	url, err := c.server.Parse("jobs")
	if err != nil {
		// This only happens when "jobs" cannot be parsed.
		panic(err)
	}

	var buf bytes.Buffer
	err = json.NewEncoder(&buf).Encode(api.RequestJobJSONRequestBody{
		Types:  types,
		Owners: &owners,
	})
	if err != nil {
		panic(err)
	}

	response, err := c.requester.Post(url.String(), "application/json", &buf)
	if err != nil {
		return nil, fmt.Errorf("error requesting job: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusCreated {
		return nil, errorFromResponse(response, "error requesting job")
	}

	var jr requestJobResponse
	err = json.NewDecoder(response.Body).Decode(&jr)
	if err != nil {
		return nil, fmt.Errorf("error parsing response: %v", err)
	}

	location, err := c.server.Parse(jr.Location)
	if err != nil {
		return nil, fmt.Errorf("error parsing location url in response: %v", err)
	}

	artifactLocation, err := c.server.Parse(jr.ArtifactLocation)
	if err != nil {
		return nil, fmt.Errorf("error parsing artifact location url in response: %v", err)
	}

	return &job{
		requester:        c.requester,
		id:               jr.Id,
		jobType:          jr.Type,
		args:             jr.Args,
		dynamicArgs:      jr.DynamicArgs,
		location:         location.String(),
		artifactLocation: artifactLocation.String(),
	}, nil
}

func (j *job) Id() uuid.UUID {
	return j.id
}

func (j *job) Type() string {
	return j.jobType
}

func (j *job) Args(args interface{}) error {
	err := json.Unmarshal(j.args, args)
	if err != nil {
		return fmt.Errorf("error parsing job arguments: %v", err)
	}
	return nil
}

func (j *job) NDynamicArgs() int {
	return len(j.dynamicArgs)
}

func (j *job) DynamicArgs(i int, args interface{}) error {
	err := json.Unmarshal(j.dynamicArgs[i], args)
	if err != nil {
		return fmt.Errorf("error parsing job arguments: %v", err)
	}
	return nil
}

func (j *job) Update(result interface{}) error {
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(api.UpdateJobJSONRequestBody{
		Result: result,
	})
	if err != nil {
		panic(err)
	}

	req, err := http.NewRequest("PATCH", j.location, &buf)
	if err != nil {
		panic(err)
	}

	req.Header.Add("Content-Type", "application/json")

	response, err := j.requester.Do(req)
	if err != nil {
		return fmt.Errorf("error fetching job info: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return errorFromResponse(response, "error setting job status")
	}

	return nil
}

func (j *job) Canceled() (bool, error) {
	response, err := j.requester.Get(j.location)
	if err != nil {
		return false, fmt.Errorf("error fetching job info: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return false, errorFromResponse(response, "error fetching job info")
	}

	var jr getJobResponse
	err = json.NewDecoder(response.Body).Decode(&jr)
	if err != nil {
		return false, fmt.Errorf("error parsing reponse: %v", err)
	}

	return jr.Canceled, nil
}

func (j *job) UploadArtifact(name string, reader io.Reader) error {
	if j.artifactLocation == "" {
		return fmt.Errorf("server does not accept artifacts for this job")
	}

	loc, err := url.Parse(j.artifactLocation)
	if err != nil {
		return fmt.Errorf("error parsing job location: %v", err)
	}

	loc, err = loc.Parse(url.PathEscape(name))
	if err != nil {
		panic(err)
	}

	req, err := http.NewRequest("PUT", loc.String(), reader)
	if err != nil {
		return fmt.Errorf("cannot create request: %v", err)
	}

	req.Header.Add("Content-Type", "application/octet-stream")

	response, err := j.requester.Do(req)
	if err != nil {
		return fmt.Errorf("error uploading artifact: %v", err)
	}

	if response.StatusCode != 200 {
		return errorFromResponse(response, "error uploading artifact")
	}

	return nil
}

// Parses an api.Error from a response and returns it as a golang error. Other
// errors, such failing to parse the response, are returned as golang error as
// well. If client code expects an error, it gets one.
func errorFromResponse(response *http.Response, message string) error {
	var e api.Error
	err := json.NewDecoder(response.Body).Decode(&e)
	if err != nil {
		return fmt.Errorf("failed to parse error response: %v", err)
	}
	return fmt.Errorf("%v: %v — %v", message, response.StatusCode, e.Message)
}
