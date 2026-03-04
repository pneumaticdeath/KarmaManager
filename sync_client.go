package main

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log"
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
	syncMu    sync.Mutex // serializes FullSync — prevents concurrent runs
	authToken string
	userID    string
	userEmail string
	prefs     fyne.Preferences
	httpClient *http.Client
	httpSem    chan struct{} // limits concurrent in-flight HTTP requests
}

const (
	prefSyncToken = "sync.auth_token"
	prefSyncUser  = "sync.user_id"
	prefSyncEmail = "sync.user_email"
)

func NewSyncClient(prefs fyne.Preferences) *SyncClient {
	sc := &SyncClient{
		prefs:      prefs,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		httpSem:    make(chan struct{}, 8),
	}
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

// DeleteAccount permanently deletes all favorites records then the user account
// on the server, then signs out locally.
func (sc *SyncClient) DeleteAccount() error {
	sc.mu.Lock()
	userID := sc.userID
	sc.mu.Unlock()

	// Fetch all records (live + tombstones) and delete them first to satisfy
	// referential integrity before deleting the user.
	allRecords, err := sc.fetchRecords("")
	if err != nil {
		return fmt.Errorf("fetching records for deletion: %w", err)
	}
	for _, r := range allRecords {
		resp, err := sc.doRequest("DELETE", "/api/collections/favorites/records/"+r.ID, nil)
		if err != nil {
			return fmt.Errorf("deleting favorite %s: %w", r.ID, err)
		}
		io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
			return fmt.Errorf("deleting favorite %s failed (%d)", r.ID, resp.StatusCode)
		}
	}

	resp, err := sc.doRequest("DELETE", "/api/collections/users/records/"+userID, nil)
	if err != nil {
		return err
	}
	data, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("delete account failed (%d): %s", resp.StatusCode, string(data))
	}
	sc.SignOut()
	return nil
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
	Page       int          `json:"page"`
	TotalPages int          `json:"totalPages"`
	Items      []pbFavorite `json:"items"`
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
	sc.httpSem <- struct{}{}
	defer func() { <-sc.httpSem }()
	return sc.httpClient.Do(req)
}

// fetchRecords pulls all favorites matching the given filter, paginating as needed.
func (sc *SyncClient) fetchRecords(filter string) ([]pbFavorite, error) {
	const perPage = 200
	var all []pbFavorite
	for page := 1; ; page++ {
		path := fmt.Sprintf("/api/collections/favorites/records?perPage=%d&page=%d&filter=%s",
			perPage, page, urlEncode(filter))
		resp, err := sc.doRequest("GET", path, nil)
		if err != nil {
			return nil, err
		}
		data, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("fetch records failed (%d): %s", resp.StatusCode, string(data))
		}
		var result pbListResult
		if err := json.Unmarshal(data, &result); err != nil {
			return nil, err
		}
		all = append(all, result.Items...)
		if page >= result.TotalPages {
			break
		}
	}
	return all, nil
}

// FullSync does a bidirectional sync of local favorites against the server.
// Only one FullSync may run at a time; a concurrent call returns immediately.
func (sc *SyncClient) FullSync(favs *FavoritesSlice) error {
	if !sc.IsAuthenticated() {
		return fmt.Errorf("not authenticated")
	}
	sc.syncMu.Lock()
	defer sc.syncMu.Unlock()

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

	// Build the set of live client_ids so the tombstone filter below can
	// ignore tombstones that share a client_id with a still-live server record.
	// This situation arises from the historical Push double-encoding bug, which
	// created multiple PB records with identical client_ids; dedup tombstoned
	// the extras, leaving tombstones whose client_id matches the canonical live
	// record and would otherwise falsely evict local favorites.
	liveClientIDs := make(map[string]bool, len(serverRecords))
	for _, r := range serverRecords {
		liveClientIDs[r.ClientID] = true
	}

	// Remove local favorites that were deleted remotely.
	// Skip any tombstone whose client_id also appears on a live server record —
	// that tombstone is a stale duplicate artifact, not an intentional deletion.
	tombstoneIDs := make(map[string]bool, len(tombstones))
	for _, r := range tombstones {
		if !liveClientIDs[r.ClientID] {
			tombstoneIDs[r.ClientID] = true
		}
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

	log.Printf("FullSync: fetched %d server records, %d tombstones, %d local favs",
		len(serverRecords), len(tombstones), len(*favs))

	canonical := make(map[contentKey]pbFavorite, len(serverRecords))
	var dedupIDs []string // PB record IDs to tombstone
	for _, r := range serverRecords {
		k := contentKey{Normalize(r.Input), Normalize(r.Anagram)}
		existing, exists := canonical[k]
		if !exists {
			canonical[k] = r
			continue
		}
		// Prefer the record already known locally; tombstone the other.
		// Use tombstoneByPBID directly since we have the PB record ID —
		// no need for an extra GET to resolve client_id → PB ID.
		if localByID[r.ClientID] && !localByID[existing.ClientID] {
			dedupIDs = append(dedupIDs, existing.ID)
			canonical[k] = r
		} else {
			dedupIDs = append(dedupIDs, r.ID)
		}
	}

	log.Printf("FullSync: canonical map has %d unique entries, %d duplicates to tombstone",
		len(canonical), len(dedupIDs))

	// Tombstone all server-side duplicates and wait for completion.
	// This runs concurrently (bounded by httpSem) but is awaited so the server
	// is actually cleaned up before this FullSync returns.
	if len(dedupIDs) > 0 {
		var dedupWg sync.WaitGroup
		for _, pbID := range dedupIDs {
			dedupWg.Add(1)
			go func(id string) {
				defer dedupWg.Done()
				if err := sc.tombstoneByPBID(id); err != nil {
					log.Printf("FullSync: tombstone %s failed: %v", id, err)
				}
			}(pbID)
		}
		dedupWg.Wait()
		log.Printf("FullSync: tombstoned %d duplicate server records", len(dedupIDs))
	}

	// --- Apply server-side edits ---
	// If a server record shares a client_id with a local favorite but has
	// different content, the favorite was edited on another device — adopt the
	// server's content so it propagates here.
	serverByClientID := make(map[string]pbFavorite, len(serverRecords))
	for _, r := range serverRecords {
		serverByClientID[r.ClientID] = r
	}
	for i, fav := range *favs {
		if r, exists := serverByClientID[fav.ID]; exists {
			if fav.Input != r.Input || fav.Anagram != r.Anagram {
				(*favs)[i].Input = r.Input
				(*favs)[i].Anagram = r.Anagram
				(*favs)[i].Dictionaries = r.Dicts
			}
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

	log.Printf("FullSync: after local dedup: %d local favs, seenLocal has %d keys",
		len(*favs), len(seenLocal))

	// Persist the deduped local slice immediately — don't wait for push/pull.
	// This ensures duplicates are removed from prefs even if network ops fail.
	SaveFavorites(*favs, sc.prefs)
	fyne.Do(RebuildFavorites)

	// Rebuild local ID set after dedup.
	localByID = make(map[string]bool, len(*favs))
	for _, fav := range *favs {
		localByID[fav.ID] = true
	}

	// Push local favorites not on server — concurrently (skip existence check
	// since canonical already tells us these aren't on the server).
	var (
		pushWg     sync.WaitGroup
		pushMu     sync.Mutex
		pushErrors []string
	)
	for _, fav := range *favs {
		if _, exists := canonical[contentKey{Normalize(fav.Input), Normalize(fav.Anagram)}]; !exists {
			pushWg.Add(1)
			go func(f FavoriteAnagram) {
				defer pushWg.Done()
				if err := sc.pushNew(f); err != nil {
					pushMu.Lock()
					pushErrors = append(pushErrors, err.Error())
					pushMu.Unlock()
				}
			}(fav)
		}
	}
	pushWg.Wait()

	// Pull server-only favorites (already deduped in canonical map).
	pulled := 0
	for k, r := range canonical {
		if !seenLocal[k] {
			log.Printf("FullSync: pulling server-only: %q / %q", r.Input, r.Anagram)
			*favs = append(*favs, FavoriteAnagram{
				Dictionaries: r.Dicts,
				Input:        r.Input,
				Anagram:      r.Anagram,
				ID:           r.ClientID,
			})
			pulled++
		}
	}
	log.Printf("FullSync: pulled %d from server; final local count %d", pulled, len(*favs))

	// Save again to include any newly-pulled server entries.
	SaveFavorites(*favs, sc.prefs)
	fyne.Do(RebuildFavorites)

	if len(pushErrors) > 0 {
		return fmt.Errorf("sync completed (%d pulled) but %d push(es) failed: %s",
			pulled, len(pushErrors), pushErrors[0])
	}
	return nil
}

// pushNew creates a new record on the server without checking for an existing
// one first. Used by FullSync's push loop where canonical already confirms the
// record is absent from the server.
func (sc *SyncClient) pushNew(fav FavoriteAnagram) error {
	if fav.ID == "" {
		fav.ID = newUUID()
	}
	dicts := fav.Dictionaries
	if dicts == "" {
		dicts = "unknown"
	}
	resp, err := sc.doRequest("POST", "/api/collections/favorites/records", map[string]any{
		"client_id":    fav.ID,
		"user":         sc.userID,
		"dictionaries": dicts,
		"input":        fav.Input,
		"anagram":      fav.Anagram,
		"deleted":      false,
	})
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

// Push upserts a single favorite on the server.
func (sc *SyncClient) Push(fav FavoriteAnagram) error {
	if !sc.IsAuthenticated() {
		return fmt.Errorf("not authenticated")
	}
	if fav.ID == "" {
		fav.ID = newUUID()
	}

	// Check if it exists on server.
	existing, err := sc.fetchRecords("client_id='" + fav.ID + "'")
	_ = err
	dicts := fav.Dictionaries
	if dicts == "" {
		dicts = "unknown"
	}
	payload := map[string]any{
		"client_id":    fav.ID,
		"user":         sc.userID,
		"dictionaries": dicts,
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

// tombstoneByPBID marks a server record as deleted by its PocketBase record ID.
// Used by FullSync's dedup loop where we already have the PB ID and don't need
// an extra GET to resolve client_id → PB ID.
//
// A new client_id is assigned at the same time. This is critical: duplicate
// records often share the same client_id as the canonical live record (a side
// effect of the old Push double-encoding bug creating multiple PB rows per
// favorite). If we tombstoned them without changing client_id, the tombstone
// filter in the next FullSync would match the canonical record's client_id and
// incorrectly remove the local favorite, causing an infinite pull-delete loop.
func (sc *SyncClient) tombstoneByPBID(pbID string) error {
	resp, err := sc.doRequest("PATCH",
		"/api/collections/favorites/records/"+pbID,
		map[string]any{
			"deleted":   true,
			"client_id": newUUID(), // prevent collision with canonical record's client_id
		})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// DeleteRemote marks a favorite as deleted on the server.
func (sc *SyncClient) DeleteRemote(clientID string) error {
	if !sc.IsAuthenticated() {
		return nil
	}
	records, err := sc.fetchRecords("client_id='" + clientID + "'")
	if err != nil || len(records) == 0 {
		return err
	}
	// Tombstone every record with this client_id (there may be duplicates from
	// the old Push bug that created multiple PB rows with the same client_id).
	for _, r := range records {
		resp, err := sc.doRequest("PATCH",
			"/api/collections/favorites/records/"+r.ID,
			map[string]any{"deleted": true})
		if err != nil {
			return err
		}
		resp.Body.Close()
	}
	return nil
}

// GenerateShareURL calls the custom API to get a share URL for a favorite.
func (sc *SyncClient) GenerateShareURL(clientID string) (string, error) {
	if !sc.IsAuthenticated() {
		return "", fmt.Errorf("not authenticated")
	}
	// Find the PocketBase record ID from client_id.
	records, err := sc.fetchRecords("client_id='" + clientID + "'")
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
