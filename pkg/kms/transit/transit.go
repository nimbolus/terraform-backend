package transit

import (
	"encoding/base64"
	"fmt"

	"github.com/hashicorp/vault/api"
)

const Name = "transit"

type VaultTransit struct {
	client *api.Client
	engine string
	key    string
}

func NewVaultTransit(client *api.Client, engine string, key string) *VaultTransit {
	return &VaultTransit{
		client: client,
		engine: engine,
		key:    key,
	}
}

func (v *VaultTransit) GetName() string {
	return Name
}

func (v *VaultTransit) Encrypt(d []byte) ([]byte, error) {
	params := map[string]any{
		"plaintext": base64.StdEncoding.EncodeToString(d),
	}
	path := fmt.Sprintf("%s/encrypt/%s", v.engine, v.key)
	res, err := v.client.Logical().Write(path, params)
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
	params := map[string]any{
		"ciphertext": string(d),
	}
	path := fmt.Sprintf("%s/decrypt/%s", v.engine, v.key)
	res, err := v.client.Logical().Write(path, params)
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
