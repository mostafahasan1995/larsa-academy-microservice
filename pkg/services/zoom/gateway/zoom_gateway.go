package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type ZoomMeetingRequest struct {
	Topic     string `json:"topic"`
	Type      int    `json:"type"` // 2 for scheduled meeting
	StartTime string `json:"start_time"`
	Duration  int    `json:"duration"`
	Timezone  string `json:"timezone"`
	Password  string `json:"password,omitempty"`
	Agenda    string `json:"agenda,omitempty"`
	Settings  struct {
		HostVideo        bool   `json:"host_video"`
		ParticipantVideo bool   `json:"participant_video"`
		JoinBeforeHost   bool   `json:"join_before_host"`
		MuteUponEntry    bool   `json:"mute_upon_entry"`
		WaitingRoom      bool   `json:"waiting_room"`
		AutoRecording    string `json:"auto_recording"` // "local", "cloud", "none"
	} `json:"settings"`
}

type ZoomMeetingResponse struct {
	Id        int64  `json:"id"`
	JoinUrl   string `json:"join_url"`
	StartUrl  string `json:"start_url"`
	Password  string `json:"password"`
	Topic     string `json:"topic"`
	StartTime string `json:"start_time"`
	Duration  int    `json:"duration"`
}

type ZoomUserResponse struct {
	Id        string `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
	Status    string `json:"status"`
}

type ZoomUserListResponse struct {
	TotalRecords  int                `json:"total_records"`
	PageSize      int                `json:"page_size"`
	NextPageToken string             `json:"next_page_token"`
	Users         []ZoomUserResponse `json:"users"`
}

type ZoomGatewayInterface interface {
	CreateMeeting(ctx context.Context, token string, userEmail string, req *ZoomMeetingRequest) (*ZoomMeetingResponse, error)
	ListUsers(ctx context.Context, token string) ([]ZoomUserResponse, error)
	DeleteMeeting(ctx context.Context, token string, meetingId string, scheduleForReminder, cancelMeetingReminder bool) error
}

type ZoomGateway struct {
	httpClient *http.Client
}

func NewZoomGateway() *ZoomGateway {
	return &ZoomGateway{
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (g *ZoomGateway) CreateMeeting(ctx context.Context, token string, userEmail string, req *ZoomMeetingRequest) (*ZoomMeetingResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("https://api.zoom.us/v2/users/%s/meetings", userEmail)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := g.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("zoom api error: status %d, body: %s", resp.StatusCode, string(respBody))
	}

	var zoomResp ZoomMeetingResponse
	if err := json.NewDecoder(resp.Body).Decode(&zoomResp); err != nil {
		return nil, err
	}

	return &zoomResp, nil
}

func (g *ZoomGateway) ListUsers(ctx context.Context, token string) ([]ZoomUserResponse, error) {
	url := "https://api.zoom.us/v2/users?status=active&page_size=300"
	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	resp, err := g.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("zoom api error: status %d, body: %s", resp.StatusCode, string(respBody))
	}

	var userList ZoomUserListResponse
	if err := json.NewDecoder(resp.Body).Decode(&userList); err != nil {
		return nil, err
	}

	return userList.Users, nil
}

func (g *ZoomGateway) DeleteMeeting(ctx context.Context, token string, meetingId string, scheduleForReminder, cancelMeetingReminder bool) error {
	url := fmt.Sprintf("https://api.zoom.us/v2/meetings/%s?schedule_for_reminder=%v&cancel_meeting_reminder=%v", meetingId, scheduleForReminder, cancelMeetingReminder)
	httpReq, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return err
	}

	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	resp, err := g.httpClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("zoom api error: status %d, body: %s", resp.StatusCode, string(respBody))
	}

	return nil
}
