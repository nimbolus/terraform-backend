package github

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/nimbolus/terraform-backend/pkg/terraform"
)

const PATName = "github_pat"

type PATAuthenticator struct {
	org string
}

func NewPATAuthenticator(org string) *PATAuthenticator {
	return &PATAuthenticator{
		org: org,
	}
}

func (pa *PATAuthenticator) GetName() string {
	return PATName
}

func makeRequest(url, token string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("invalid status: %d", resp.StatusCode)
	}
	return resp, nil
}

type identity struct {
	Login string `json:"login"`
}

func (pa *PATAuthenticator) Authenticate(secret string, s *terraform.State) (bool, error) {
	// check access to repo that matches project in org
	_, err := makeRequest(fmt.Sprintf("https://api.github.com/repos/%s/%s", pa.org, s.Project), secret)
	if err == nil {
		// allow when there is no error
		return true, nil
	}

	// check if org matches username
	resp, err := makeRequest("https://api.github.com/user", secret)
	if err != nil {
		return false, err
	}

	var user identity
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return false, err
	}
	if user.Login == pa.org {
		return true, nil
	}

	// check if user belongs to org
	resp, err = makeRequest("https://api.github.com/user/orgs", secret)
	if err != nil {
		return false, err
	}

	var orgs []identity
	if err := json.NewDecoder(resp.Body).Decode(&orgs); err != nil {
		return false, err
	}

	for _, org := range orgs {
		if org.Login == pa.org {
			return true, nil
		}
	}

	return false, nil
}
