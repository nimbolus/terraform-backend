package vault

import (
	"fmt"
	"os"

	vault "github.com/hashicorp/vault/api"
	"github.com/spf13/viper"
)

const (
	k8sServiceAccountFile = "/var/run/secrets/kubernetes.io/serviceaccount/token"
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

	if token := viper.GetString("vault_token"); token != "" {
		client.SetToken(token)
	} else if role := viper.GetString("vault_kube_auth_role"); role != "" {
		jwt, err := os.ReadFile(k8sServiceAccountFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read k8s service account: %w", err)
		}

		viper.SetDefault("vault_kube_auth_name", "kubernetes")
		path := fmt.Sprintf("auth/%s/login", viper.GetString("vault_kube_auth_name"))
		params := map[string]interface{}{
			"jwt":  string(jwt),
			"role": role,
		}
		secret, err := client.Logical().Write(path, params)
		if err != nil {
			return nil, fmt.Errorf("failed to login with k8s service account: %w", err)
		}

		client.SetToken(secret.Auth.ClientToken)
	} else {
		return nil, fmt.Errorf("unable to initialize vault client: no login method found")
	}

	return client, nil
}

func GetKvValue(client *vault.Client, path string, value string) (string, error) {
	secret, err := client.Logical().Read(path)
	if err != nil {
		return "", fmt.Errorf("failed to get vault secret at %s: %v", path, err)
	}

	data, ok := secret.Data["data"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("failed to get vault secret data")
	}

	key, ok := data[value].(string)
	if !ok {
		return "", fmt.Errorf("failed to get vault secret key value")
	}

	return key, nil
}
