package trackerclient

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/go-retryablehttp"
)

const defaultTrackerUrl = "https://legacy-api.arpa.li"

type TrackerConfig struct {
	Project        string
	ProjectVersion string
	TrackerUrl     string
	Username       string
	Password       string
	httpClient     *retryablehttp.Client
}

type TrackerClient struct {
	trackerConfig *TrackerConfig
}

type Item struct {
}

func NewTrackerConfig(trackerConfig *TrackerConfig) (*TrackerClient, error) {
	trackerConfig.httpClient = retryablehttp.NewClient()
	if trackerConfig.TrackerUrl == "" {
		trackerConfig.TrackerUrl = defaultTrackerUrl
	}
	// if last char of tracker url is '/', remove it
	if trackerConfig.TrackerUrl[len(trackerConfig.TrackerUrl)-1] == '/' {
		trackerConfig.TrackerUrl = trackerConfig.TrackerUrl[:len(trackerConfig.TrackerUrl)-1]
	}
	var err error
	trackerConfig.Project = strings.TrimSpace(trackerConfig.Project)
	if trackerConfig.Project == "" {
		err = multierror.Append(err, fmt.Errorf("option must not be empty: Project"))
	}
	trackerConfig.ProjectVersion = strings.TrimSpace(trackerConfig.ProjectVersion)
	if trackerConfig.ProjectVersion == "" {
		err = multierror.Append(err, fmt.Errorf("option must not be empty: ProjectVersion"))
	}
	trackerConfig.Username = strings.TrimSpace(trackerConfig.Username)
	if trackerConfig.Username == "" {
		err = multierror.Append(err, fmt.Errorf("option must not be empty: Username"))
	}
	if err != nil {
		return nil, err
	}
	return &TrackerClient{
		trackerConfig: trackerConfig,
	}, nil
}

func (that *TrackerClient) newRequest(m string, p string, b any) (*retryablehttp.Request, error) {
	url := fmt.Sprintf("%s/%s/%s", that.trackerConfig.TrackerUrl, that.trackerConfig.Project, p)
	req, err := retryablehttp.NewRequest(m, url, b)
	if err != nil {
		return nil, err
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("user-agent", fmt.Sprintf("go-trackerclient %s/%s", that.trackerConfig.Project, that.trackerConfig.ProjectVersion))
	req.Header.Set("ateam-tracker-project", that.trackerConfig.Project)
	req.Header.Set("ateam-tracker-user", that.trackerConfig.Username)
	req.Header.Set("ateam-tracker-version", that.trackerConfig.ProjectVersion)
	if that.trackerConfig.Password != "" {
		req.SetBasicAuth(that.trackerConfig.Username, that.trackerConfig.Password)
	}
	return req, nil
}

type requestItemsRequest struct {
	Downloader string `json:"downloader"`
	APIVersion string `json:"api_version"`
	Version    string `json:"version"`
}

type requestItemsResponse struct {
	Items  []string `json:"items"`
	Queues []string `json:"queues"`
}

func (that *TrackerClient) RequestItems(limit uint64) ([]string, error) {
	if limit < 1 {
		return nil, fmt.Errorf("limit must be greater than 0")
	}
	p := "request"
	if limit > 1 {
		p = fmt.Sprintf("multi=%d/request", limit)
	}
	reqBody, err := json.Marshal(&requestItemsRequest{
		Downloader: that.trackerConfig.Username,
		APIVersion: "2",
		Version:    that.trackerConfig.ProjectVersion,
	})
	if err != nil {
		return nil, err
	}
	req, err := that.newRequest(http.MethodPost, p, reqBody)
	if err != nil {
		return nil, err
	}
	res, err := that.trackerConfig.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode == 404 || res.StatusCode == 204 {
		return nil, ErrNoTasksAvailable
	}
	if res.StatusCode == 404 {
		return nil, ErrNoSuchProject
	}
	if res.StatusCode >= 300 {
		return nil, fmt.Errorf("%s: %d", ErrInvalidTrackerResponse, res.StatusCode)
	}
	var response requestItemsResponse
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		return nil, err
	}
	return response.Items, nil
}

func (that *TrackerClient) RequestItem() (item string, err error) {
	items, err := that.RequestItems(1)
	if err != nil {
		return "", err
	}
	if len(items) == 0 {
		return "", nil
	}
	return items[0], nil
}

type itemsDoneRequest struct {
	Downloader string            `json:"downloader"`
	Version    string            `json:"version"`
	Items      []string          `json:"items"`
	Bytes      map[string]uint64 `json:"bytes"`
}

func (that *TrackerClient) ItemsDone(items []string, bytes map[string]uint64) error {
	if len(items) == 0 {
		return nil
	}
	reqBody, err := json.Marshal(&itemsDoneRequest{
		Downloader: that.trackerConfig.Username,
		Version:    that.trackerConfig.ProjectVersion,
		Items:      items,
		Bytes:      bytes,
	})
	if err != nil {
		return err
	}
	req, err := that.newRequest(http.MethodPost, "done", reqBody)
	if err != nil {
		return err
	}
	res, err := that.trackerConfig.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode == 404 {
		return ErrNoSuchProject
	}
	if res.StatusCode >= 300 {
		return fmt.Errorf("%s: %d", ErrInvalidTrackerResponse, res.StatusCode)
	}
	return nil
}
