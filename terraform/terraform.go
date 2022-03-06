package terraform

import (
	"crypto/sha256"
	"fmt"
)

type State struct {
	ID   string
	Data []byte
	Lock []byte
}

func GetStateID(project, id string) string {
	path := fmt.Sprintf("%s-%s", project, id)
	hash := sha256.Sum256([]byte(path))
	return fmt.Sprintf("%x", hash[:])
}
