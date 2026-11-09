package token

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"larsa-academy-microservice/pkg/caching"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
	Scope       string `json:"scope"`
}

type TokenManager struct {
	httpClient *http.Client
}

func NewTokenManager() *TokenManager {
	return &TokenManager{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (tm *TokenManager) GetToken(ctx context.Context, accountId, clientId, clientSecret string, forceRefresh bool) (string, error) {
	cacheKey := fmt.Sprintf("zoom:token:%s", clientId)

	// 1. Try Cache (unless forceRefresh is true)
	if caching.Rdb != nil && !forceRefresh {
		if cached, err := caching.Rdb.GetByKey(cacheKey); err == nil && cached != "" {
			return cached, nil
		}
	}

	// 2. Fetch from Zoom
	tokenRes, err := tm.fetchToken(ctx, accountId, clientId, clientSecret)
	if err != nil {
		return "", err
	}

	// 3. Save to Cache (subtract 60s for safety)
	if caching.Rdb != nil {
		ttl := time.Duration(tokenRes.ExpiresIn-60) * time.Second
		if ttl > 0 {
			caching.Rdb.CacheByKey(cacheKey, tokenRes.AccessToken, ttl)
		}
	}

	return tokenRes.AccessToken, nil
}

func (tm *TokenManager) fetchToken(ctx context.Context, accountId, clientId, clientSecret string) (*TokenResponse, error) {
	data := url.Values{}
	data.Set("grant_type", "account_credentials")
	data.Set("account_id", accountId)

	req, err := http.NewRequestWithContext(ctx, "POST", "https://zoom.us/oauth/token", strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(clientId, clientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := tm.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("zoom token error: status %d, body: %s", resp.StatusCode, string(body))
	}

	var tokenRes TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenRes); err != nil {
		return nil, err
	}

	return &tokenRes, nil
}
