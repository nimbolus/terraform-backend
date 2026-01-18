package vault

import (
	"fmt"
	"os"
	"sync"

	vault "github.com/hashicorp/vault/api"
	"github.com/spf13/viper"

	"github.com/nimbolus/terraform-backend/internal"
	log "github.com/sirupsen/logrus"
)

const (
	k8sServiceAccountFile = "/var/run/secrets/kubernetes.io/serviceaccount/token"
)

var (
	authTokenMutex sync.Mutex
)

func NewVaultClient() (*vault.Client, error) {
	config := vault.DefaultConfig()
	if config.Address = viper.GetString("vault_addr"); config.Address == "" {
		return nil, fmt.Errorf("unable to initialize vault client: no vault address defined")
	}

	client, err := vault.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("unable to initialize vault client: %w", err)
	}

	token, err := internal.SecretEnvOrFile("vault_token", "vault_token_file")
	if err != nil {
		return nil, fmt.Errorf("getting vault token: %w", err)
	}

	if token != "" {
		client.SetToken(token)
	} else if role := viper.GetString("vault_kube_auth_role"); role != "" {
		secret, err := kubeAuthLogin(client, role)
		if err != nil {
			return nil, err
		}

		go func() {
			for {
				if err := authTokenWatcher(client, secret); err != nil {
					log.Fatal(err.Error())
				}

				secret, err = kubeAuthLogin(client, role)
				if err != nil {
					log.Fatal(err.Error())
				}
			}
		}()

	} else {
		return nil, fmt.Errorf("unable to initialize vault client: no login method found")
	}

	return client, nil
}

func kubeAuthLogin(client *vault.Client, role string) (*vault.Secret, error) {
	authTokenMutex.Lock()
	defer authTokenMutex.Unlock()

	jwt, err := os.ReadFile(k8sServiceAccountFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read k8s service account: %w", err)
	}

	viper.SetDefault("vault_kube_auth_name", "kubernetes")
	path := fmt.Sprintf("auth/%s/login", viper.GetString("vault_kube_auth_name"))
	params := map[string]any{
		"jwt":  string(jwt),
		"role": role,
	}

	log.Infof("log in to vault using k8s service account with role %s", role)
	secret, err := client.Logical().Write(path, params)
	if err != nil {
		return nil, fmt.Errorf("failed to login with k8s service account: %w", err)
	}

	client.SetToken(secret.Auth.ClientToken)
	return secret, nil
}

func authTokenWatcher(client *vault.Client, secret *vault.Secret) error {
	watcher, err := client.NewLifetimeWatcher(&vault.LifetimeWatcherInput{
		Secret: secret,
	})
	if err != nil {
		return fmt.Errorf("failed to initialize vault auth token lifetime watcher: %v", err)
	}

	go watcher.Start()
	defer watcher.Stop()

	for {
		select {
		case info := <-watcher.RenewCh():
			log.Infof("vault auth token was renewed successfully, remaining lifetime %ds", info.Secret.Auth.LeaseDuration)
		case _ = <-watcher.DoneCh():
			log.Warnf("vault auth token could not be renewed, try reauthentication")
			return nil
		}
	}
}

func GetKvValue(client *vault.Client, path string, value string) (string, error) {
	authTokenMutex.Lock()
	defer authTokenMutex.Unlock()

	secret, err := client.Logical().Read(path)
	if err != nil {
		return "", fmt.Errorf("failed to get vault secret at %s: %v", path, err)
	}

	data, ok := secret.Data["data"].(map[string]any)
	if !ok {
		return "", fmt.Errorf("failed to get vault secret data")
	}

	key, ok := data[value].(string)
	if !ok {
		return "", fmt.Errorf("failed to get vault secret key value")
	}

	return key, nil
}
