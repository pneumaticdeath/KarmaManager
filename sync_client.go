package main

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"fyne.io/fyne/v2"
)

const syncBaseURL = "https://karmamanager-sync.fly.dev"

// SyncSvc is the global sync client, initialized in main().
var SyncSvc *SyncClient

type SyncClient struct {
	mu        sync.Mutex
	authToken string
	userID    string
	userEmail string
	prefs     fyne.Preferences
}

const (
	prefSyncToken = "sync.auth_token"
	prefSyncUser  = "sync.user_id"
	prefSyncEmail = "sync.user_email"
)

func NewSyncClient(prefs fyne.Preferences) *SyncClient {
	sc := &SyncClient{prefs: prefs}
	sc.authToken = prefs.String(prefSyncToken)
	sc.userID = prefs.String(prefSyncUser)
	sc.userEmail = prefs.String(prefSyncEmail)
	return sc
}

func (sc *SyncClient) IsAuthenticated() bool {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	return sc.authToken != ""
}

func (sc *SyncClient) UserEmail() string {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	return sc.userEmail
}

// RequestOTP sends an OTP to email, returns the otpId needed to verify.
func (sc *SyncClient) RequestOTP(email string) (string, error) {
	body, _ := json.Marshal(map[string]string{"email": email})
	resp, err := http.Post(
		syncBaseURL+"/api/collections/users/request-otp",
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("request OTP failed (%d): %s", resp.StatusCode, string(data))
	}
	var result struct {
		OtpID string `json:"otpId"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", err
	}
	return result.OtpID, nil
}

// AuthWithOTP verifies the OTP code and stores the auth token.
func (sc *SyncClient) AuthWithOTP(otpID, code string) error {
	body, _ := json.Marshal(map[string]string{"otpId": otpID, "password": code})
	resp, err := http.Post(
		syncBaseURL+"/api/collections/users/auth-with-otp",
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("auth failed (%d): %s", resp.StatusCode, string(data))
	}
	var result struct {
		Token  string `json:"token"`
		Record struct {
			ID    string `json:"id"`
			Email string `json:"email"`
		} `json:"record"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return err
	}
	sc.mu.Lock()
	sc.authToken = result.Token
	sc.userID = result.Record.ID
	sc.userEmail = result.Record.Email
	sc.mu.Unlock()
	sc.prefs.SetString(prefSyncToken, result.Token)
	sc.prefs.SetString(prefSyncUser, result.Record.ID)
	sc.prefs.SetString(prefSyncEmail, result.Record.Email)
	return nil
}

// SignOut clears stored credentials.
func (sc *SyncClient) SignOut() {
	sc.mu.Lock()
	sc.authToken = ""
	sc.userID = ""
	sc.userEmail = ""
	sc.mu.Unlock()
	sc.prefs.SetString(prefSyncToken, "")
	sc.prefs.SetString(prefSyncUser, "")
	sc.prefs.SetString(prefSyncEmail, "")
}

// pbFavorite is the JSON shape returned by PocketBase for a favorites record.
type pbFavorite struct {
	ID          string `json:"id"`
	ClientID    string `json:"client_id"`
	Dicts       string `json:"dictionaries"`
	Input       string `json:"input"`
	Anagram     string `json:"anagram"`
	ShareToken  string `json:"share_token"`
	Deleted     bool   `json:"deleted"`
}

type pbListResult struct {
	Items []pbFavorite `json:"items"`
}

func (sc *SyncClient) token() string {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	return sc.authToken
}

func (sc *SyncClient) doRequest(method, path string, body any) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		bodyReader = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, syncBaseURL+path, bodyReader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", sc.token())
	client := &http.Client{Timeout: 15 * time.Second}
	return client.Do(req)
}

// fetchAllRecords pulls all favorites for the authenticated user with the given filter.
func (sc *SyncClient) fetchRecords(filter string) ([]pbFavorite, error) {
	path := fmt.Sprintf("/api/collections/favorites/records?perPage=500&filter=%s",
		urlEncode(filter))
	resp, err := sc.doRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch records failed (%d): %s", resp.StatusCode, string(data))
	}
	var result pbListResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result.Items, nil
}

// FullSync does a bidirectional sync of local favorites against the server.
func (sc *SyncClient) FullSync(favs *FavoritesSlice) error {
	if !sc.IsAuthenticated() {
		return fmt.Errorf("not authenticated")
	}

	type contentKey struct{ input, anagram string }

	// Pull live records.
	serverRecords, err := sc.fetchRecords("deleted=false")
	if err != nil {
		return fmt.Errorf("sync pull failed: %w", err)
	}

	// Pull tombstones.
	tombstones, err := sc.fetchRecords("deleted=true")
	if err != nil {
		return fmt.Errorf("sync tombstones failed: %w", err)
	}

	// Remove local favorites that were deleted remotely.
	tombstoneIDs := make(map[string]bool, len(tombstones))
	for _, r := range tombstones {
		tombstoneIDs[r.ClientID] = true
	}
	filtered := make(FavoritesSlice, 0, len(*favs))
	for _, fav := range *favs {
		if !tombstoneIDs[fav.ID] {
			filtered = append(filtered, fav)
		}
	}
	*favs = filtered

	// --- Server-side dedup ---
	// For each content key, elect one canonical server record. Prefer records
	// whose client_id is already referenced locally (avoids ID churn).
	// Tombstone all non-canonical duplicates immediately.
	localByID := make(map[string]bool, len(*favs))
	for _, fav := range *favs {
		localByID[fav.ID] = true
	}

	canonical := make(map[contentKey]pbFavorite, len(serverRecords))
	for _, r := range serverRecords {
		k := contentKey{Normalize(r.Input), Normalize(r.Anagram)}
		existing, exists := canonical[k]
		if !exists {
			canonical[k] = r
			continue
		}
		// Prefer the record already known locally; tombstone the other.
		if localByID[r.ClientID] && !localByID[existing.ClientID] {
			go sc.DeleteRemote(existing.ClientID)
			canonical[k] = r
		} else {
			go sc.DeleteRemote(r.ClientID)
		}
	}

	// --- Local dedup ---
	// Deduplicate local slice by content, adopting canonical server client_id.
	seenLocal := make(map[contentKey]bool, len(*favs))
	deduped := make(FavoritesSlice, 0, len(*favs))
	for _, fav := range *favs {
		k := contentKey{Normalize(fav.Input), Normalize(fav.Anagram)}
		if seenLocal[k] {
			continue // local duplicate — drop silently
		}
		seenLocal[k] = true
		if r, exists := canonical[k]; exists {
			fav.ID = r.ClientID // adopt canonical server ID
		}
		deduped = append(deduped, fav)
	}
	*favs = deduped

	// Rebuild local ID set after dedup.
	localByID = make(map[string]bool, len(*favs))
	for _, fav := range *favs {
		localByID[fav.ID] = true
	}

	// Push local favorites not on server.
	for _, fav := range *favs {
		if _, exists := canonical[contentKey{Normalize(fav.Input), Normalize(fav.Anagram)}]; !exists {
			_ = sc.Push(fav) // best-effort
		}
	}

	// Pull server-only favorites (already deduped in canonical map).
	for k, r := range canonical {
		if !seenLocal[k] {
			*favs = append(*favs, FavoriteAnagram{
				Dictionaries: r.Dicts,
				Input:        r.Input,
				Anagram:      r.Anagram,
				ID:           r.ClientID,
			})
		}
	}

	SaveFavorites(*favs, sc.prefs)
	fyne.Do(RebuildFavorites)
	return nil
}

// Push upserts a single favorite on the server.
func (sc *SyncClient) Push(fav FavoriteAnagram) error {
	if !sc.IsAuthenticated() {
		return fmt.Errorf("not authenticated")
	}
	if fav.ID == "" {
		fav.ID = newUUID()
	}

	// Check if it exists on server.
	existing, err := sc.fetchRecords(urlEncode("client_id='"+fav.ID+"'"))
	_ = err
	payload := map[string]any{
		"client_id":    fav.ID,
		"user":         sc.userID,
		"dictionaries": fav.Dictionaries,
		"input":        fav.Input,
		"anagram":      fav.Anagram,
		"deleted":      false,
	}

	var resp *http.Response
	if len(existing) > 0 {
		resp, err = sc.doRequest("PATCH",
			"/api/collections/favorites/records/"+existing[0].ID, payload)
	} else {
		resp, err = sc.doRequest("POST", "/api/collections/favorites/records", payload)
	}
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("push failed (%d): %s", resp.StatusCode, string(data))
	}
	return nil
}

// DeleteRemote marks a favorite as deleted on the server.
func (sc *SyncClient) DeleteRemote(clientID string) error {
	if !sc.IsAuthenticated() {
		return nil
	}
	records, err := sc.fetchRecords(urlEncode("client_id='" + clientID + "'"))
	if err != nil || len(records) == 0 {
		return err
	}
	resp, err := sc.doRequest("PATCH",
		"/api/collections/favorites/records/"+records[0].ID,
		map[string]any{"deleted": true})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// GenerateShareURL calls the custom API to get a share URL for a favorite.
func (sc *SyncClient) GenerateShareURL(clientID string) (string, error) {
	if !sc.IsAuthenticated() {
		return "", fmt.Errorf("not authenticated")
	}
	// Find the PocketBase record ID from client_id.
	records, err := sc.fetchRecords(urlEncode("client_id='" + clientID + "'"))
	if err != nil {
		return "", err
	}
	if len(records) == 0 {
		// Push it first, then retry.
		return "", fmt.Errorf("favorite not found on server — sync first")
	}
	pbID := records[0].ID
	resp, err := sc.doRequest("POST", "/api/ext/favorites/"+pbID+"/share", nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("share failed (%d): %s", resp.StatusCode, string(data))
	}
	var result struct {
		ShareURL string `json:"share_url"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", err
	}
	return result.ShareURL, nil
}

// newUUID returns a random UUID v4 string.
func newUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// urlEncode does minimal percent-encoding for PocketBase filter query params.
func urlEncode(s string) string {
	// net/url.QueryEscape would work but turns spaces into '+'; use PathEscape
	// for PocketBase filter strings.
	result := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9',
			c == '-', c == '_', c == '.', c == '~',
			c == '=', c == '!', c == '<', c == '>', c == '\'':
			result = append(result, c)
		default:
			result = append(result, fmt.Sprintf("%%%02X", c)...)
		}
	}
	return string(result)
}
