package slave

import "fmt"

type WebsocketAPI struct {
	s *SlaveBackend
}

func NewWebsocketAPI(s *SlaveBackend) *WebsocketAPI {
	return &WebsocketAPI{s: s}
}

func (w *WebsocketAPI) NewHeads() {
	fmt.Sprintf("---------------- NewHeads")
}

func (w *WebsocketAPI) Logs() {
	fmt.Println("----------------- logs")
}

func (w *WebsocketAPI) NewPendingTransactions() {}

func (w *WebsocketAPI) Syncing() {}
