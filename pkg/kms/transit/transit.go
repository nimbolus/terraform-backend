package transit

import (
	"encoding/base64"
	"fmt"

	vaultclient "github.com/nimbolus/terraform-backend/pkg/client/vault"
)

const Name = "transit"

type VaultTransit struct {
	engine string
	key    string
}

func NewVaultTransit(engine string, key string) (*VaultTransit, error) {
	_, err := vaultclient.NewVaultClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create vault transit client: %v", err)
	}

	return &VaultTransit{
		engine: engine,
		key:    key,
	}, nil
}

func (v *VaultTransit) GetName() string {
	return Name
}

func (v *VaultTransit) Encrypt(d []byte) ([]byte, error) {
	client, err := vaultclient.NewVaultClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create vault transit client: %v", err)
	}

	params := map[string]interface{}{
		"plaintext": base64.StdEncoding.EncodeToString(d),
	}
	path := fmt.Sprintf("%s/encrypt/%s", v.engine, v.key)
	res, err := client.Logical().Write(path, params)
	if err != nil {
		return nil, fmt.Errorf("failed to seal with transit engine: %v", err)
	}

	ciphertext, ok := res.Data["ciphertext"].(string)
	if !ok {
		return nil, fmt.Errorf("failed to get ciphertext")
	}

	return []byte(ciphertext), nil
}

func (v *VaultTransit) Decrypt(d []byte) ([]byte, error) {
	client, err := vaultclient.NewVaultClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create vault transit client: %v", err)
	}

	params := map[string]interface{}{
		"ciphertext": string(d),
	}
	path := fmt.Sprintf("%s/decrypt/%s", v.engine, v.key)
	res, err := client.Logical().Write(path, params)
	if err != nil {
		return nil, fmt.Errorf("failed to unseal with transit engine: %v", err)
	}

	plaintext, ok := res.Data["plaintext"].(string)
	if !ok {
		return nil, fmt.Errorf("failed to get plaintext")
	}

	data, err := base64.StdEncoding.DecodeString(plaintext)
	if err != nil {
		return nil, fmt.Errorf("failed to decode state data: %v", err)
	}

	return data, nil
}
