package terraform

import (
	"crypto/sha256"
	"fmt"
)

type State struct {
	ID      string
	Data    []byte
	Lock    LockInfo
	Project string
	Name    string
}

func GetStateID(project, id string) string {
	path := fmt.Sprintf("%s-%s", project, id)
	hash := sha256.Sum256([]byte(path))
	return fmt.Sprintf("%x", hash[:])
}

type LockInfo struct {
	ID        string `json:"ID"`
	Path      string `json:"Path"`
	Operation string `json:"Operation"`
	Who       string `json:"Who"`
	Version   string `json:"Version"`
	Created   string `json:"Created"`
	Info      string `json:"Info"`
}

func (l LockInfo) Equal(r LockInfo) bool {
	return l.ID == r.ID &&
		l.Path == r.Path &&
		l.Operation == r.Operation &&
		l.Who == r.Who &&
		l.Version == r.Version &&
		l.Created == r.Created &&
		l.Info == r.Info
}
