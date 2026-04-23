// Package pve is a minimal Proxmox PVE2 JSON API client (API token / ticket auth).
//
// Official reference for paths and parameters: [PVE API viewer]
// (https://pve.proxmox.com/pve-docs/api-viewer/) — e.g. GET/POST /api2/json/nodes,
// /nodes/{node}/qemu, /nodes/{node}/lxc, vncproxy, version.
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
		// Accept self-signed, snake-oil, and hostname/IP mismatch vs cert SANs (lab PVE).
		// Clone default transport to keep connection pooling, HTTP/2, etc.
		//nolint:gosec // G402: org explicitly disables verification for dev / internal clusters
		t := http.DefaultTransport.(*http.Transport).Clone()
		t.TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true,
			MinVersion:         tls.VersionTLS12,
		}
		tr = t
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

// ListLxc lists LXC guests on a node.
func (c *Client) ListLxc(node string) ([]VMListItem, error) {
	p := "/api2/json/nodes/" + url.PathEscape(node) + "/lxc"
	resp, err := c.get(p)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("pve lxc: %s: %s", resp.Status, string(b))
	}
	var out struct {
		Data []VMListItem `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out.Data, nil
}

// VncProxy creates a VNC/websocket session for QEMU or LXC and returns the ticket + port
// (see PVE2 API: POST /nodes/{node}/qemu|lxc/{vmid}/vncproxy).
func (c *Client) VncProxy(node string, vmid int, lxc bool) (port int, ticket string, err error) {
	var p string
	if lxc {
		p = fmt.Sprintf("/api2/json/nodes/%s/lxc/%d/vncproxy", url.PathEscape(node), vmid)
	} else {
		p = fmt.Sprintf("/api2/json/nodes/%s/qemu/%d/vncproxy", url.PathEscape(node), vmid)
	}
	// Proxmox expects websocket mode for the browser noVNC client
	v := url.Values{}
	v.Set("websocket", "1")
	resp, err := c.postForm(p, v)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return 0, "", fmt.Errorf("pve vncproxy: %s: %s", resp.Status, string(b))
	}
	var out struct {
		Data *struct {
			Port   int    `json:"port"`
			Ticket string `json:"ticket"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return 0, "", err
	}
	if out.Data == nil {
		return 0, "", fmt.Errorf("pve vncproxy: empty data")
	}
	return out.Data.Port, out.Data.Ticket, nil
}

// NvcNoV1 builds the noVNC URL that Proxmox’s web server serves, matching the pattern used by
// external UIs: POST vncproxy, then point the browser at 8006 with ?console= &path= to vncwebsocket
// (see e.g. https://github.com/zzantares/ProxmoxVE/issues/17).
// The user’s browser must be able to reach c.BaseURL (cluster API URL) over HTTPS; this is a direct
// link to the PVE UI’s embedded noVNC, not a stream through Cloudmanager.
func (c *Client) NvcNoV1URL(node string, vmid int, lxcGuest bool, port int, ticket string) (string, error) {
	if port == 0 || ticket == "" {
		return "", fmt.Errorf("missing port or vnc ticket")
	}
	pSeg := "qemu"
	console := "kvm"
	if lxcGuest {
		pSeg = "lxc"
		console = "lxc"
	}
	u0, err := url.Parse(c.BaseURL)
	if err != nil {
		return "", err
	}
	innerQ := url.Values{}
	innerQ.Set("port", fmt.Sprint(port))
	innerQ.Set("vncticket", ticket)
	inner := fmt.Sprintf("api2/json/nodes/%s/%s/%d/vncwebsocket?%s", node, pSeg, vmid, innerQ.Encode())
	outer := url.Values{}
	outer.Set("console", console)
	outer.Set("novnc", "1")
	outer.Set("node", node)
	outer.Set("vmid", fmt.Sprint(vmid))
	outer.Set("resize", "scale")
	outer.Set("path", inner)
	u0.RawPath = ""
	u0.Path = "/"
	u0.RawQuery = outer.Encode()
	return u0.String(), nil
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
