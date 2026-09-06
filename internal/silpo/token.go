package silpo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/voloshmiak/silpo-agent-backend/internal/storage"
)

// TokenPair holds a refreshed access+refresh token from Silpo.
type TokenPair struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    *time.Time
}

// Service handles Silpo token validation and refresh.
type Service struct {
	refreshURL string
	httpClient *http.Client
}

func NewService(refreshURL string) *Service {
	return &Service{
		refreshURL: refreshURL,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

// refreshToken calls the Silpo custom refresh endpoint.
// TODO: adjust the request/response structure once the endpoint details are confirmed.
func (s *Service) refreshToken(ctx context.Context, refreshToken string) (*TokenPair, error) {
	if s.refreshURL == "" {
		return nil, fmt.Errorf("SILPO_REFRESH_URL is not configured")
	}

	body, _ := json.Marshal(map[string]string{"refresh_token": refreshToken})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.refreshURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("silpo refresh request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("silpo refresh returned status %d", resp.StatusCode)
	}

	// Adjust field names to match the actual Silpo response.
	var result struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"` // seconds
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode silpo refresh response: %w", err)
	}

	pair := &TokenPair{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
	}
	if result.ExpiresIn > 0 {
		exp := time.Now().Add(time.Duration(result.ExpiresIn) * time.Second)
		pair.ExpiresAt = &exp
	}
	return pair, nil
}

// isExpired checks whether a stored token has already expired (with a 30s buffer).
func isExpired(t *storage.SilpoToken) bool {
	if t.ExpiresAt == nil {
		return false // no expiry info — assume valid
	}
	return time.Now().After(t.ExpiresAt.Add(-30 * time.Second))
}

// EnsureValid returns a valid Silpo access token for the user, refreshing if needed.
func (s *Service) EnsureValid(ctx context.Context, repo *storage.TokenRepo, userID uuid.UUID) (string, error) {
	t, err := repo.GetByUserID(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("no silpo token for user %s: %w", userID, err)
	}

	if !isExpired(t) {
		return t.AccessToken, nil
	}

	// Token is expired — refresh it.
	if t.RefreshToken == "" {
		return "", fmt.Errorf("silpo token expired and no refresh token available")
	}

	pair, err := s.refreshToken(ctx, t.RefreshToken)
	if err != nil {
		return "", fmt.Errorf("silpo token refresh failed: %w", err)
	}

	// Save the new tokens.
	updated := &storage.SilpoToken{
		UserID:       userID,
		AccessToken:  pair.AccessToken,
		RefreshToken: pair.RefreshToken,
		ExpiresAt:    pair.ExpiresAt,
	}
	if err := repo.Upsert(ctx, updated); err != nil {
		return "", fmt.Errorf("save refreshed silpo token: %w", err)
	}

	return pair.AccessToken, nil
}
