package speculative

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/hashicorp/go-slug"
	"github.com/nimbolus/terraform-backend/pkg/tfcontext"
)

func serveWorkspace(ctx context.Context) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	backend, err := tfcontext.FindBackend(cwd)
	if err != nil {
		return "", err
	}
	backendURL, err := url.Parse(backend.Address)
	if err != nil {
		return "", fmt.Errorf("failed to parse backend url: %s, %w", backend.Address, err)
	}
	if backend.Password == "" {
		backendPassword, ok := os.LookupEnv("TF_HTTP_PASSWORD")
		if !ok || backendPassword == "" {
			return "", errors.New("missing backend password")
		}
		backend.Password = backendPassword
	}

	id := uuid.New()
	backendURL.Path = filepath.Join(backendURL.Path, "/share/", id.String())

	pr, pw := io.Pipe()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, backendURL.String(), pr)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	req.SetBasicAuth(backend.Username, backend.Password)

	go func() {
		_, err := slug.Pack(cwd, pw, true)
		if err != nil {
			fmt.Printf("failed to pack workspace: %v\n", err)
			pw.CloseWithError(err)
		} else {
			pw.Close()
		}
	}()

	go func() {
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			fmt.Printf("failed to stream workspace: %v\n", err)
		} else if resp.StatusCode/100 != 2 {
			fmt.Printf("invalid status code after streaming workspace: %d\n", resp.StatusCode)
		}
		fmt.Println("done streaming workspace")
	}()

	return backendURL.String(), nil
}
