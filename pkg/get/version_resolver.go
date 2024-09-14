package get

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/alexellis/arkade/pkg"
)

// Tools can implement version resolver to resolve versions differently
type VersionResolver interface {
	Version() (string, error)
}

type GithubVersionResolver struct {
	Owner string
	Repo  string
}

func (r *GithubVersionResolver) Version() (string, error) {
	url := fmt.Sprintf("https://github.com/%s/%s/releases/latest", r.Owner, r.Repo)
	client := makeHTTPClient(&githubTimeout, false)
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

	req, err := http.NewRequest(http.MethodHead, url, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("User-Agent", pkg.UserAgent())

	res, err := client.Do(req)
	if err != nil {
		return "", err
	}

	if res.Body != nil {
		defer res.Body.Close()
	}

	if res.StatusCode != http.StatusMovedPermanently && res.StatusCode != http.StatusFound {
		return "", fmt.Errorf("server returned status: %d", res.StatusCode)
	}

	loc := res.Header.Get("Location")
	if len(loc) == 0 {
		return "", fmt.Errorf("unable to determine release of tool")
	}

	version := loc[strings.LastIndex(loc, "/")+1:]
	return version, nil
}

type K8VersionResolver struct{}

func (r *K8VersionResolver) Version() (string, error) {
	url := "https://cdn.dl.k8s.io/release/stable.txt"

	timeout := time.Second * 5
	client := makeHTTPClient(&timeout, false)
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("User-Agent", pkg.UserAgent())

	res, err := client.Do(req)
	if err != nil {
		return "", err
	}

	if res.Body == nil {
		return "", fmt.Errorf("unable to determine release of tool")
	}

	defer res.Body.Close()
	bodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}

	version := string(bodyBytes)
	return version, nil
}

type GoVersionResolver struct{}

func (r *GoVersionResolver) Version() (string, error) {
	req, err := http.NewRequest(http.MethodGet, "https://go.dev/VERSION?m=text", nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("User-Agent", pkg.UserAgent())

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}

	if res.Body == nil {
		return "", fmt.Errorf("unexpected empty body")
	}

	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)

	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", res.StatusCode)
	}

	content := strings.TrimSpace(string(body))
	exp, err := regexp.Compile(`^go(\d+.\d+.\d+)`)
	if err != nil {
		return "", err
	}

	version := exp.FindStringSubmatch(content)
	if len(version) < 2 {
		return "", fmt.Errorf("failed to fetch go latest version number")
	}

	return version[1], nil
}
