//go:build integration || handler
// +build integration handler

package server

import (
	"bytes"
	"encoding/json"
	"flag"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"

	"github.com/gorilla/mux"
	"github.com/gruntwork-io/terratest/modules/terraform"

	localkms "github.com/nimbolus/terraform-backend/pkg/kms/local"
	locallock "github.com/nimbolus/terraform-backend/pkg/lock/local"
	"github.com/nimbolus/terraform-backend/pkg/storage/filesystem"
)

func NewStateHandler(t *testing.T) http.Handler {
	store, err := filesystem.NewFileSystemStorage(filepath.Join("./handler_test", "storage"))
	if err != nil {
		t.Fatal(err)
	}

	locker := locallock.NewLock()

	key := "x8DiIkAKRQT7cF55NQLkAZk637W3bGVOUjGeMX5ZGXY="
	kms, _ := localkms.NewKMS(key)

	r := mux.NewRouter().StrictSlash(true)
	r.HandleFunc("/state/{project}/{name}", StateHandler(store, locker, kms))

	return r
}

var terraformBinary = flag.String("tf", "terraform", "terraform binary")

func terraformOptions(t *testing.T, addr string) *terraform.Options {
	return terraform.WithDefaultRetryableErrors(t, &terraform.Options{
		TerraformDir:    "./handler_test",
		TerraformBinary: *terraformBinary,
		Vars:            map[string]interface{}{},
		Reconfigure:     true,
		BackendConfig: map[string]interface{}{
			"address":        addr,
			"lock_address":   addr,
			"unlock_address": addr,
			"username":       "basic",
			"password":       "some-random-secret",
		},
		LockTimeout: "200ms",
		Lock:        true,
	})
}

func TestServerHandler_VerifyLockOnPush(t *testing.T) {
	s := httptest.NewServer(NewStateHandler(t))
	defer s.Close()

	address, err := url.JoinPath(s.URL, "/state/project1/example")
	if err != nil {
		t.Fatal(err)
	}

	simulateLock(t, address, true)

	for _, doLock := range []bool{true, false} {
		terraformOptions := terraformOptions(t, address)
		terraformOptions.Lock = doLock

		_, err = terraform.InitAndApplyE(t, terraformOptions)
		if err == nil {
			t.Fatal("expected error")
		}

		simulateLock(t, address, false)
	}
}

func TestServerHandler(t *testing.T) {
	s := httptest.NewServer(NewStateHandler(t))
	defer s.Close()

	address, err := url.JoinPath(s.URL, "/state/project1/example")
	if err != nil {
		t.Fatal(err)
	}

	terraformOptions := terraformOptions(t, address)

	// Clean up resources with "terraform destroy" at the end of the test.
	defer terraform.Destroy(t, terraformOptions)

	// Run "terraform init" and "terraform apply". Fail the test if there are any errors.
	terraform.InitAndApply(t, terraformOptions)

	simulateLock(t, address, true)

	_, err = terraform.ApplyE(t, terraformOptions)
	if err == nil {
		t.Fatal("expected error")
	}

	simulateLock(t, address, false)

	terraform.ApplyAndIdempotent(t, terraformOptions)
}

func simulateLock(t *testing.T, address string, lock bool) {
	method := "LOCK"
	if !lock {
		method = "UNLOCK"
	}

	postBody, _ := json.Marshal(map[string]string{
		"ID":        "cf290ef3-6090-410e-9784-d017a4b1536a",
		"Path":      "",
		"Operation": "simulateLock",
		"Who":       "simulator",
		"Version":   "0.0.0",
		"Created":   "2021-01-01T00:00:00Z",
		"Info":      "",
	})

	req, err := http.NewRequest(method, address, bytes.NewBuffer(postBody))
	if err != nil {
		t.Fatal(err)
	}

	req.SetBasicAuth("basic", "some-random-secret")

	if _, err := http.DefaultClient.Do(req); err != nil {
		t.Fatal(err)
	}
}
