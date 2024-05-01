//go:build integration || handler
// +build integration handler

package server

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/gruntwork-io/terratest/modules/terraform"
)

func TestStateMigration(t *testing.T) {
	baseDir := "./state_migration_test"

	s := httptest.NewServer(NewStateHandler(t, baseDir, true))
	defer s.Close()

	address, err := url.JoinPath(s.URL, "/state/project1/example")
	if err != nil {
		t.Fatal(err)
	}

	terraformOptions := terraformOptions(t, baseDir, address)
	terraformOptions.Reconfigure = false
	terraformOptions.Lock = true
	backendConf := terraformOptions.BackendConfig

	// init with http backend
	if err := os.WriteFile(filepath.Join(baseDir, "backend.tf"), []byte("terraform {\n  backend \"http\" {}\n}"), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := terraform.InitAndApplyE(t, terraformOptions); err != nil {
		t.Fatal(err)
	}

	// init -migrate-state to local backend
	terraformOptions.MigrateState = true
	terraformOptions.BackendConfig = map[string]interface{}{}

	if err := os.WriteFile(filepath.Join(baseDir, "backend.tf"), []byte("terraform {\n  backend \"local\" {}\n}"), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := terraform.InitAndApplyE(t, terraformOptions); err != nil {
		t.Fatal(err)
	}

	// init -migrate-state to http backend
	terraformOptions.BackendConfig = backendConf

	if err := os.WriteFile(filepath.Join(baseDir, "backend.tf"), []byte("terraform {\n  backend \"http\" {}\n}"), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := terraform.InitAndApplyE(t, terraformOptions); err != nil {
		t.Fatal(err)
	}

	// destroy
	if _, err := terraform.DestroyE(t, terraformOptions); err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequest(http.MethodDelete, address, nil)
	if err != nil {
		t.Fatal(err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte("basic:some-random-secret"))))

	if _, err := http.DefaultClient.Do(req); err != nil {
		t.Fatal(err)
	}

	// verify state file is deleted
	if _, err := os.Stat(filepath.Join(baseDir, "storage/d82238e1158b32f0b445c5da058608a8c1d83551f890b19b7e90d78cce1a808d.tfstate")); !os.IsNotExist(err) {
		t.Fatal("state file should be deleted")
	}

	// cleanup
	for _, f := range []string{"terraform.tfstate", ".terraform/terraform.tfstate"} {
		if err := os.Remove(filepath.Join(baseDir, f)); err != nil {
			t.Fatal(err)
		}
	}
}
