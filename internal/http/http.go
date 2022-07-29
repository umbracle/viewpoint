package http

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
)

// HttpClient is an http over the standard http interface
type HttpClient struct {
	addr string
}

// NewHttpClient creates a new http client
func NewHttpClient(addr string) *HttpClient {
	return &HttpClient{addr: addr}
}

func (h *HttpClient) get(url string, objResp interface{}) error {
	fullURL := h.addr + url

	resp, err := http.Get(fullURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	var obj struct {
		Data json.RawMessage
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}

	if err := json.Unmarshal(obj.Data, &objResp); err != nil {
		return err
	}
	return nil
}

type Identity struct {
	PeerID string `json:"peer_id"`
	ENR    string `json:"enr"`
}

// NodeIdentity returns the node network identity
func (h *HttpClient) NodeIdentity() (*Identity, error) {
	var out *Identity
	err := h.get("/eth/v1/node/identity", &out)
	return out, err
}

type Syncing struct {
	HeadSlot      string
	SyncDistance  string
	IsSyncing     bool
	IsOptimisitic bool
}

func (h *HttpClient) Syncing() (*Syncing, error) {
	var out *Syncing
	err := h.get("/eth/v1/node/syncing", &out)
	return out, err
}
