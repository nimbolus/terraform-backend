package vaultclient

import (
	"fmt"

	vault "github.com/hashicorp/vault/api"
	"github.com/spf13/viper"
)

func NewVaultClient() (*vault.Client, error) {
	config := vault.DefaultConfig()
	config.Address = viper.GetString("vault_addr")

	client, err := vault.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("unable to initialize vault client: %v", err)
	}

	if token := viper.GetString("vault_token"); token != "" {
		client.SetToken(token)
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
