package terraform

type State struct {
	ID   string
	Data []byte
	Lock []byte
}
