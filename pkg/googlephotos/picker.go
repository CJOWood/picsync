package googlephotos

import (
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// PollingConfig contains recommended polling settings returned by the API
// to control how often we should poll and when to stop polling.
type PollingConfig struct {
	PollInterval string `json:"pollInterval"`
	TimeoutIn    string `json:"timeoutIn"`
}

// PickingSession represents a session from the Picker API.
type PickingSession struct {
	ID            string         `json:"id"`
	PickerURI     string         `json:"pickerUri"`
	PollingConfig *PollingConfig `json:"pollingConfig"`
	MediaItemsSet bool           `json:"mediaItemsSet"`
}

// PickedMediaItem contains basic information about a picked item.
type PickedMediaItem struct {
	ID       string `json:"id"`
	BaseURL  string `json:"baseUrl"`
	MimeType string `json:"mimeType"`
	Filename string `json:"filename"`
}

type listMediaItemsResponse struct {
	MediaItems    []PickedMediaItem `json:"mediaItems"`
	NextPageToken string            `json:"nextPageToken"`
}

// CreatePickingSession creates a new picking session.
func CreatePickingSession(c *http.Client) (*PickingSession, error) {
	body := `{}`
	resp := PickingSession{}
	err := PostUnmarshalJSON(c, "https://photospicker.googleapis.com/v1/sessions", body, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetPickingSession retrieves information about a session.
func GetPickingSession(c *http.Client, id string) (*PickingSession, error) {
	resp := PickingSession{}
	u := fmt.Sprintf("https://photospicker.googleapis.com/v1/sessions/%s", url.PathEscape(id))
	err := GetUnmarshalJSON(c, u, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// WaitForPicking waits until media items have been selected for the session
// using the recommended polling configuration returned by the API.
func WaitForPicking(c *http.Client, id string, pc *PollingConfig) error {
	pollInterval := 5 * time.Second
	if pc != nil {
		if d, err := time.ParseDuration(pc.PollInterval); err == nil && d > 0 {
			pollInterval = d
		}
	}
	timeout := time.Duration(0)
	if pc != nil {
		if d, err := time.ParseDuration(pc.TimeoutIn); err == nil {
			timeout = d
		}
	}
	deadline := time.Time{}
	if timeout > 0 {
		deadline = time.Now().Add(timeout)
	}

	for {
		sess, err := GetPickingSession(c, id)
		if err != nil {
			return err
		}
		if sess.MediaItemsSet {
			return nil
		}
		if !deadline.IsZero() && time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for session")
		}
		// Use any updated polling config in the response.
		if sess.PollingConfig != nil {
			if d, err := time.ParseDuration(sess.PollingConfig.PollInterval); err == nil && d > 0 {
				pollInterval = d
			}
			if d, err := time.ParseDuration(sess.PollingConfig.TimeoutIn); err == nil {
				timeout = d
				if timeout > 0 {
					deadline = time.Now().Add(timeout)
				}
			}
		}
		time.Sleep(pollInterval)
	}
}

// ListPickedMediaItems lists the media items chosen by the user.
func ListPickedMediaItems(c *http.Client, id string) ([]PickedMediaItem, error) {
	u := fmt.Sprintf("https://photospicker.googleapis.com/v1/mediaItems?sessionId=%s", url.QueryEscape(id))
	resp := listMediaItemsResponse{}
	err := GetUnmarshalJSON(c, u, &resp)
	if err != nil {
		return nil, err
	}
	return resp.MediaItems, nil
}
