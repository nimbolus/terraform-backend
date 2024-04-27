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
	"os"
	"path/filepath"
	"testing"

	"github.com/gorilla/mux"
	"github.com/gruntwork-io/terratest/modules/terraform"

	localkms "github.com/nimbolus/terraform-backend/pkg/kms/local"
	"github.com/nimbolus/terraform-backend/pkg/lock"
	locallock "github.com/nimbolus/terraform-backend/pkg/lock/local"
	"github.com/nimbolus/terraform-backend/pkg/storage/filesystem"
	tf "github.com/nimbolus/terraform-backend/pkg/terraform"
)

var terraformBinary = flag.String("tf", "terraform", "terraform binary")

func NewStateHandler(t *testing.T, baseDir string, forceUnlockEnabled bool) http.Handler {
	store, err := filesystem.NewFileSystemStorage(filepath.Join(baseDir, "storage"))
	if err != nil {
		t.Fatal(err)
	}

	var locker lock.Locker = locallock.NewLock()
	if forceUnlockEnabled {
		locker = lock.NewLockerWithForceUnlockEnabled(locker)
	}

	key := "x8DiIkAKRQT7cF55NQLkAZk637W3bGVOUjGeMX5ZGXY="
	kms, _ := localkms.NewKMS(key)

	r := mux.NewRouter().StrictSlash(true)
	r.HandleFunc("/state/{project}/{name}", StateHandler(store, locker, kms))

	return r
}

func terraformOptions(t *testing.T, baseDir, addr string) *terraform.Options {
	return terraform.WithDefaultRetryableErrors(t, &terraform.Options{
		TerraformDir:    baseDir,
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
	s := httptest.NewServer(NewStateHandler(t, "./handler_test", true))
	defer s.Close()

	address, err := url.JoinPath(s.URL, "/state/project1/example")
	if err != nil {
		t.Fatal(err)
	}

	simulateLock(t, address, true)

	for _, doLock := range []bool{true, false} {
		terraformOptions := terraformOptions(t, "./handler_test", address)
		terraformOptions.Lock = doLock

		_, err = terraform.InitAndApplyE(t, terraformOptions)
		if err == nil {
			t.Fatal("expected error")
		}

		simulateLock(t, address, false)
	}
}

func TestServerHandler(t *testing.T) {
	s := httptest.NewServer(NewStateHandler(t, "./handler_test", true))
	defer s.Close()

	address, err := url.JoinPath(s.URL, "/state/project1/example")
	if err != nil {
		t.Fatal(err)
	}

	terraformOptions := terraformOptions(t, "./handler_test", address)

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

	if err := os.Remove("./handler_test/errored.tfstate"); err != nil {
		t.Fatal(err)
	}
}

func TestServerHandler_ForceUnlock_Enabled(t *testing.T) {
	s := httptest.NewServer(NewStateHandler(t, "./handler_test", true))
	defer s.Close()

	address, err := url.JoinPath(s.URL, "/state/project1/example")
	if err != nil {
		t.Fatal(err)
	}

	terraformOptions := terraformOptions(t, "./handler_test", address)

	terraform.Init(t, terraformOptions)

	simulateLock(t, address, true)

	if _, err := terraform.RunTerraformCommandE(t, terraformOptions, "force-unlock", "-force", "cf290ef3-6090-410e-9784-d017a4b1536a"); err != nil {
		t.Fatal(err)
	}
}

func TestServerHandler_ForceUnlock_Disabled(t *testing.T) {
	s := httptest.NewServer(NewStateHandler(t, "./handler_test", false))
	defer s.Close()

	address, err := url.JoinPath(s.URL, "/state/project1/example")
	if err != nil {
		t.Fatal(err)
	}

	terraformOptions := terraformOptions(t, "./handler_test", address)

	terraform.Init(t, terraformOptions)

	simulateLock(t, address, true)

	if _, err := terraform.RunTerraformCommandE(t, terraformOptions, "force-unlock", "-force", "cf290ef3-6090-410e-9784-d017a4b1536a"); err == nil {
		t.Fatal("expected error")
	}

	simulateLock(t, address, false)
}

func simulateLock(t *testing.T, address string, doLock bool) {
	method := "LOCK"
	if !doLock {
		method = "UNLOCK"
	}

	postBody, _ := json.Marshal(&tf.LockInfo{
		ID:        "cf290ef3-6090-410e-9784-d017a4b1536a",
		Path:      "",
		Operation: "simulateLock",
		Who:       "simulator",
		Version:   "0.0.0",
		Created:   "2021-01-01T00:00:00Z",
		Info:      "",
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
