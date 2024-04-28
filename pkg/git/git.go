package git

import (
	"errors"
	"net/url"
	"os"

	"github.com/go-git/go-git/v5"
	giturls "github.com/whilp/git-urls"
)

func RepoOrigin() (*url.URL, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	repo, err := git.PlainOpen(cwd)
	if err != nil {
		return nil, err
	}

	orig, err := repo.Remote("origin")
	if err != nil {
		return nil, err
	}
	if orig == nil {
		return nil, errors.New("origin remote not present")
	}

	for _, u := range orig.Config().URLs {
		remoteURL, err := giturls.Parse(u)
		if err != nil {
			continue
		}
		if remoteURL.Hostname() == "github.com" {
			return remoteURL, nil
		}
	}
	return nil, errors.New("no suitable url found")
}
