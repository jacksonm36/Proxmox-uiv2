// Package pve is a minimal Proxmox PVE2 API client (ticket / API token).
package pve

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	BaseURL  string
	HTTP     *http.Client
	User     string
	TokenID  string
	Secret   string
	Insecure bool
}

type NodeStatus struct {
	Node string
}

type VMListItem struct {
	VMID  int    `json:"vmid"`
	Name  string `json:"name"`
	Status string `json:"status"`
	Type  string `json:"type"`
}

func NewClient(baseURL, user, tokenID, secret string, verifyTLS bool) *Client {
	var tr http.RoundTripper
	if !verifyTLS {
		tr = &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}} //nolint:gosec // org opt-out of TLS verify
	}
	return &Client{
		BaseURL:  strings.TrimRight(baseURL, "/"),
		HTTP:     &http.Client{Timeout: 60 * time.Second, Transport: tr},
		User:     user,
		TokenID:  tokenID,
		Secret:   secret,
		Insecure: !verifyTLS,
	}
}

func (c *Client) authHeader() string {
	// PVEAPIToken=user@realm!tokenid=secret
	return fmt.Sprintf("PVEAPIToken=%s!%s=%s", c.User, c.TokenID, c.Secret)
}

func (c *Client) get(path string) (*http.Response, error) {
	u, err := url.JoinPath(c.BaseURL, path)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", c.authHeader())
	return c.HTTP.Do(req)
}

func (c *Client) postForm(path string, v url.Values) (*http.Response, error) {
	u, err := url.JoinPath(c.BaseURL, path)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, u, strings.NewReader(v.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", c.authHeader())
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return c.HTTP.Do(req)
}

// ListNodes returns node names the token can see.
func (c *Client) ListNodes() ([]string, error) {
	resp, err := c.get("/api2/json/nodes")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("pve nodes: %s: %s", resp.Status, string(b))
	}
	var out struct {
		Data []struct {
			Node string `json:"node"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	names := make([]string, 0, len(out.Data))
	for _, n := range out.Data {
		names = append(names, n.Node)
	}
	return names, nil
}

// ListQemu lists QEMU/KVM on a node.
func (c *Client) ListQemu(node string) ([]VMListItem, error) {
	p := "/api2/json/nodes/" + url.PathEscape(node) + "/qemu"
	resp, err := c.get(p)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("pve qemu: %s: %s", resp.Status, string(b))
	}
	var out struct {
		Data []VMListItem `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out.Data, nil
}

// SetPower issues start, stop, reset, or shutdown to a guest.
func (c *Client) SetPower(node string, vmid int, action string) error {
	// /nodes/{node}/qemu/{vmid}/status/{start|stop|reset|resume|suspend|shutdown|reboot}
	act := action
	if act == "poweron" {
		act = "start"
	}
	if act == "poweroff" {
		act = "stop"
	}
	p := fmt.Sprintf("/api2/json/nodes/%s/qemu/%d/status/%s", url.PathEscape(node), vmid, url.PathEscape(act))
	resp, err := c.postForm(p, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("pve power: %s: %s", resp.Status, string(b))
	}
	return nil
}

// CloneTemplate clones a template to a new VM.
func (c *Client) CloneTemplate(node string, sourceVMID, newid int, name, pool, storage string) error {
	// POST /nodes/{node}/qemu/{vmid}/clone
	p := fmt.Sprintf("/api2/json/nodes/%s/qemu/%d/clone", url.PathEscape(node), sourceVMID)
	v := url.Values{}
	v.Set("newid", fmt.Sprint(newid))
	if name != "" {
		v.Set("name", name)
	}
	if pool != "" {
		v.Set("pool", pool)
	}
	if storage != "" {
		v.Set("storage", storage)
	}
	if storage == "" {
		v.Set("full", "0")
	}
	resp, err := c.postForm(p, v)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("pve clone: %s: %s", resp.Status, string(b))
	}
	return nil
}

// VerifyReachability is a no-op that hits version endpoint.
func (c *Client) VerifyReachability() error {
	u, _ := url.JoinPath(c.BaseURL, "/api2/json/version")
	req, _ := http.NewRequest(http.MethodGet, u, nil)
	req.Header.Set("Authorization", c.authHeader())
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("pve version: %s: %s", resp.Status, string(b))
	}
	return nil
}
