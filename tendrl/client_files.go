package tendrl

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// FileResult is the result of a successful file upload. The file is scanned by
// Surface before becoming downloadable; Status is the terminal scan state
// ("clean", or "awaiting_fetch" for tag-routed uploads) on success.
type FileResult struct {
	TransferID  string `json:"transfer_id"`
	Status      string `json:"status"`
	Mode        string `json:"mode,omitempty"` // direct | tag | group | cross_account
	FileName    string `json:"file_name"`
	Size        int64  `json:"size"`
	SHA256      string `json:"sha256"`
	ThreatLevel string `json:"threat_level"`
	// Group broadcast only:
	Delivered int      `json:"delivered,omitempty"`
	Skipped   []string `json:"skipped,omitempty"`
}

// SendFile uploads a file from disk and routes it by dest or tags. Pass exactly one:
//   - dest = a bare entity name or full "account:region:entity:name" resource path.
//     A same-account entity is a direct transfer; an entity-group dest broadcasts to
//     its members; a different-account dest is a cross-account transfer (the
//     recipient must have opted in and allowlisted this account).
//   - tags = routing tags; the file is handed to matching Strand automations.
//
// The upload is scanned synchronously and the terminal result is returned. A non-2xx
// response (402 credits, 403 not accepted, 415 type, 422 blocked) is returned as an error.
// Optional trailing meta attaches a custom JSON object to the transfer (e.g. a
// clip's {"zone":"driveway","trigger":"PIR"}); it's non-breaking for existing callers.
func (c *Client) SendFile(path, dest string, tags []string, meta ...map[string]any) (*FileResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return c.SendFileBytes(filepath.Base(path), data, dest, tags, meta...)
}

// SendFileBytes uploads raw bytes as a named file. See SendFile.
func (c *Client) SendFileBytes(filename string, data []byte, dest string, tags []string, meta ...map[string]any) (*FileResult, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("file", filename)
	if err != nil {
		return nil, err
	}
	if _, err := part.Write(data); err != nil {
		return nil, err
	}
	if dest != "" {
		_ = w.WriteField("dest", dest)
	}
	if len(tags) > 0 {
		_ = w.WriteField("tags", strings.Join(tags, ","))
	}
	if len(meta) > 0 && meta[0] != nil {
		if mb, err := json.Marshal(meta[0]); err == nil {
			_ = w.WriteField("meta", string(mb))
		}
	}
	if err := w.Close(); err != nil {
		return nil, err
	}

	endpoint, err := url.JoinPath(c.baseURL, "/entities/files")
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", endpoint, &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("send file failed (%d): %s", resp.StatusCode, string(body))
	}
	var out FileResult
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CheckFiles lists clean files available to this entity (the receiver inbox).
func (c *Client) CheckFiles(limit int) ([]map[string]any, error) {
	if limit <= 0 {
		limit = 50
	}
	endpoint, err := url.JoinPath(c.baseURL, "/entities/files")
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("GET", endpoint+"?limit="+strconv.Itoa(limit), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("check files failed (%d)", resp.StatusCode)
	}
	var out struct {
		Files []map[string]any `json:"files"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out.Files, nil
}

// DownloadFile downloads a clean file's bytes by transferID. For
// delete_on_download files (the default), a successful download consumes the
// file server-side.
func (c *Client) DownloadFile(transferID string) ([]byte, error) {
	endpoint, err := url.JoinPath(c.baseURL, "/entities/files/download/", transferID)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("download failed (%d): %s", resp.StatusCode, string(body))
	}
	return io.ReadAll(resp.Body)
}

// RescanResult is the recipient's own re-scan verdict for a received cross-account
// file. The cost is billed to the recipient's Surface credits.
type RescanResult struct {
	TransferID           string `json:"transfer_id"`
	RecipientThreatLevel string `json:"recipient_threat_level"`
	Threat               string `json:"threat"`
	Blocked              bool   `json:"blocked"`
}

// RescanFile re-scans a received cross-account file with this account's own Surface
// profile, billed to this (the recipient's) account. Only the recipient of a
// cross-account file may call it. A Blocked result means the recipient's stricter
// profile flagged the file and it is no longer downloadable on this side.
func (c *Client) RescanFile(transferID string) (*RescanResult, error) {
	endpoint, err := url.JoinPath(c.baseURL, "/entities/files", transferID, "rescan")
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("rescan failed (%d): %s", resp.StatusCode, string(body))
	}
	var out RescanResult
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
