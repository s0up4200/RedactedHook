package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/inhies/go-bytesize"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
	"golang.org/x/time/rate"

	"github.com/s0up4200/redactedhook/internal/config"
)

const (
	APIEndpointBaseRedacted = "https://redacted.ch/ajax.php"
	APIEndpointBaseOrpheus  = "https://orpheus.network/ajax.php"
	Pathhook                = "/hook"
)

const ( // HTTP status codes for custom logic
	StatusUploaderNotAllowed = http.StatusIMUsed + 1
	StatusLabelNotAllowed    = http.StatusIMUsed + 2
	StatusSizeNotAllowed     = http.StatusIMUsed + 3
	StatusRatioNotAllowed    = http.StatusIMUsed
)

var (
	redactedLimiter *rate.Limiter
	orpheusLimiter  *rate.Limiter
)

func init() {
	redactedLimiter = rate.NewLimiter(rate.Every(10*time.Second), 10)
	orpheusLimiter = rate.NewLimiter(rate.Every(10*time.Second), 5)
}

type RequestData struct {
	REDUserID   int               `json:"red_user_id,omitempty"`
	OPSUserID   int               `json:"ops_user_id,omitempty"`
	TorrentID   int               `json:"torrent_id,omitempty"`
	REDKey      string            `json:"red_apikey,omitempty"`
	OPSKey      string            `json:"ops_apikey,omitempty"`
	MinRatio    float64           `json:"minratio,omitempty"`
	MinSize     bytesize.ByteSize `json:"minsize,omitempty"`
	MaxSize     bytesize.ByteSize `json:"maxsize,omitempty"`
	Uploaders   string            `json:"uploaders,omitempty"`
	RecordLabel string            `json:"record_labels,omitempty"`
	Mode        string            `json:"mode,omitempty"`
	Indexer     string            `json:"indexer"`
	TorrentName string            `json:"torrentname,omitempty"`
}

type ResponseData struct {
	Status   string `json:"status"`
	Error    string `json:"error"`
	Response struct {
		Username string `json:"username"`
		Stats    struct {
			Ratio float64 `json:"ratio"`
		} `json:"stats"`
		Group struct {
			Name      string `json:"name"`
			MusicInfo struct {
				Artists []struct {
					ID   int    `json:"id"`
					Name string `json:"name"`
				} `json:"artists"`
			} `json:"musicInfo"`
		} `json:"group"`
		Torrent *struct {
			Username        string `json:"username"`
			Size            int64  `json:"size"`
			RecordLabel     string `json:"remasterRecordLabel"`
			ReleaseName     string `json:"filePath"`
			CatalogueNumber string `json:"remasterCatalogueNumber"`
		} `json:"torrent"`
	} `json:"response"`
}

// fetchAPI does a rate-limited API call and unmarshals the response.
func fetchAPI(endpoint, apiKey string, limiter *rate.Limiter, indexer string, target interface{}) error {
	if !limiter.Allow() {
		log.Warn().Msgf("%s: Too many requests", indexer)
		return fmt.Errorf("too many requests")
	}

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		log.Error().Msgf("fetchAPI error: %v", err)
	}
	req.Header.Set("Authorization", apiKey)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req = req.WithContext(ctx)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Error().Msgf("fetchAPI error: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error().Msgf("fetchAPI error: %v", err)
	}

	if err := json.Unmarshal(respBody, target); err != nil {
		log.Error().Msgf("fetchAPI error: %v", err)
	}

	responseData := target.(*ResponseData)
	if responseData.Status != "success" {
		log.Warn().Msgf("API error from %s: %s", indexer, responseData.Error)
		return fmt.Errorf("API error from %s: %s", indexer, responseData.Error)
	}

	return nil
}

func fetchTorrentDataIfNeeded(requestData *RequestData, torrentData **ResponseData, apiBase string) error {
	// If torrentData is already fetched, do nothing
	if *torrentData != nil {
		return nil
	}

	var apiKey string
	switch requestData.Indexer {
	case "redacted":
		apiKey = requestData.REDKey
	case "ops":
		apiKey = requestData.OPSKey
	default:
		return fmt.Errorf("invalid indexer: %s", requestData.Indexer)
	}

	var err error
	*torrentData, err = fetchTorrentData(requestData.TorrentID, apiKey, apiBase, requestData.Indexer)
	if err != nil {
		return fmt.Errorf("error fetching torrent data: %w", err)
	}
	return nil
}

func fetchTorrentData(torrentID int, apiKey, apiBase, indexer string) (*ResponseData, error) {
	limiter := getLimiter(indexer)
	if limiter == nil {
		return nil, fmt.Errorf("could not get rate limiter for indexer: %s", indexer)
	}

	endpoint := fmt.Sprintf("%s?action=torrent&id=%d", apiBase, torrentID)
	responseData := &ResponseData{}
	if err := fetchAPI(endpoint, apiKey, limiter, indexer, responseData); err != nil {
		return nil, err
	}

	// Log the release information
	if responseData.Response.Torrent != nil {
		releaseName := responseData.Response.Torrent.ReleaseName
		uploader := responseData.Response.Torrent.Username
		log.Debug().Msgf("[%s] Checking release: %s - (Uploader: %s) (TorrentID: %d)", indexer, releaseName, uploader, torrentID)
	}

	return responseData, nil
}

func fetchUserDataIfNeeded(requestData *RequestData, userData **ResponseData, apiBase string) error {
	if *userData != nil {
		return nil
	}

	var userID int
	var apiKey string
	switch requestData.Indexer {
	case "redacted":
		userID = requestData.REDUserID
		apiKey = requestData.REDKey
	case "ops":
		userID = requestData.OPSUserID
		apiKey = requestData.OPSKey
	default:
		log.Error().Str("indexer", requestData.Indexer).Msg("Invalid indexer")
		return fmt.Errorf("invalid indexer: %s", requestData.Indexer)
	}

	if userID == 0 {
		log.Error().Str("indexer", requestData.Indexer).Msg("User ID is missing but required when minratio is set")
		return fmt.Errorf("user ID is missing for indexer: %s", requestData.Indexer)
	}

	var err error
	*userData, err = fetchUserData(userID, apiKey, requestData.Indexer, apiBase)
	if err != nil {
		log.Error().Err(err).Str("indexer", requestData.Indexer).Msg("Error fetching user data")
		return fmt.Errorf("error fetching user data: %w", err)
	}
	return nil
}

func fetchUserData(userID int, apiKey, indexer, apiBase string) (*ResponseData, error) {
	limiter := getLimiter(indexer)
	endpoint := fmt.Sprintf("%s?action=user&id=%d", apiBase, userID)
	responseData := &ResponseData{}
	if err := fetchAPI(endpoint, apiKey, limiter, indexer, responseData); err != nil {
		return nil, err
	}
	return responseData, nil
}

func getLimiter(indexer string) *rate.Limiter {
	switch indexer {
	case "redacted":
		return redactedLimiter
	case "ops":
		return orpheusLimiter
	default:
		log.Error().Msgf("Invalid indexer: %s", indexer)
		return nil
	}
}

func HookData(w http.ResponseWriter, r *http.Request) {

	var torrentData *ResponseData
	var userData *ResponseData
	var requestData RequestData

	cfg := config.GetConfig()

	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is supported", http.StatusBadRequest)
		return
	}

	// Read JSON payload from the request body
	body, err := io.ReadAll(r.Body)
	defer r.Body.Close()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = json.Unmarshal(body, &requestData)
	if err != nil {
		log.Error().Msgf("[%s] Failed to unmarshal JSON payload: %s", requestData.Indexer, err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if requestData.Indexer != "ops" && requestData.Indexer != "redacted" {
		if requestData.Indexer == "" {
			log.Error().Msg("No indexer provided")
			http.Error(w, "no indexer provided", http.StatusBadRequest)
		} else {
			log.Error().Msgf("Invalid indexer: %s", requestData.Indexer)
			http.Error(w, fmt.Sprintf("Invalid indexer: %s", requestData.Indexer), http.StatusBadRequest)
		}
		return
	}

	// Check each field in requestData and fallback to config if empty
	if requestData.REDUserID == 0 {
		requestData.REDUserID = cfg.UserIDs.REDUserID
	}
	if requestData.OPSUserID == 0 {
		requestData.OPSUserID = cfg.UserIDs.OPSUserID
	}
	if requestData.REDKey == "" {
		requestData.REDKey = cfg.APIKeys.REDKey
	}
	if requestData.OPSKey == "" {
		requestData.OPSKey = cfg.APIKeys.OPSKey
	}
	if requestData.MinRatio == 0 {
		requestData.MinRatio = cfg.Ratio.MinRatio
	}
	if requestData.MinSize == 0 {
		requestData.MinSize = bytesize.ByteSize(cfg.ParsedSizes.MinSize)
	}
	if requestData.MaxSize == 0 {
		requestData.MaxSize = bytesize.ByteSize(cfg.ParsedSizes.MaxSize)
	}
	if requestData.Uploaders == "" {
		requestData.Uploaders = cfg.Uploaders.Uploaders
	}
	if requestData.Mode == "" {
		requestData.Mode = cfg.Uploaders.Mode
	}

	// Log request received
	logMsg := fmt.Sprintf("Received data request from %s", r.RemoteAddr)
	log.Info().Msg(logMsg)

	// Determine the appropriate API base based on the requested hook path
	var apiBase string
	switch requestData.Indexer {
	case "redacted":
		apiBase = APIEndpointBaseRedacted
	case "ops":
		apiBase = APIEndpointBaseOrpheus
	default:
		http.Error(w, "Invalid path", http.StatusNotFound)
		return
	}

	reqHeader := make(http.Header)
	var apiKey string
	if requestData.Indexer == "redacted" {
		apiKey = requestData.REDKey
	} else if requestData.Indexer == "ops" {
		apiKey = requestData.OPSKey
	}
	reqHeader.Set("Authorization", apiKey)

	err = viper.Unmarshal(&cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("Unable to decode into struct")
	}

	// hook uploader
	if requestData.TorrentID != 0 && requestData.Uploaders != "" {
		if err := fetchTorrentDataIfNeeded(&requestData, &torrentData, apiBase); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		username := torrentData.Response.Torrent.Username
		usernames := strings.Split(requestData.Uploaders, ",")

		for i, username := range usernames { // Trim whitespace from each username
			usernames[i] = strings.TrimSpace(username)
		}
		usernamesStr := strings.Join(usernames, ", ") // Join the usernames with a comma and a single space
		log.Trace().Msgf("[%s] Requested uploaders [%s]: %s", requestData.Indexer, requestData.Mode, usernamesStr)

		isListed := false
		for _, uname := range usernames {
			if uname == username {
				isListed = true
				break
			}
		}

		if (requestData.Mode == "blacklist" && isListed) || (requestData.Mode == "whitelist" && !isListed) {
			http.Error(w, "Uploader is not allowed", StatusUploaderNotAllowed)
			log.Debug().Msgf("[%s] Uploader (%s) is not allowed", requestData.Indexer, username)
			return
		}
	}

	// hook record label
	if requestData.TorrentID != 0 && requestData.RecordLabel != "" {
		if err := fetchTorrentDataIfNeeded(&requestData, &torrentData, apiBase); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		recordLabel := torrentData.Response.Torrent.RecordLabel
		name := torrentData.Response.Group.Name
		//releaseName := torrentData.Response.Torrent.ReleaseName
		requestedRecordLabels := strings.Split(requestData.RecordLabel, ",")

		if recordLabel == "" {
			log.Debug().Msgf("[%s] No record label found for release: %s", requestData.Indexer, name)
			http.Error(w, "Record label not allowed", StatusLabelNotAllowed)
			return
		}

		recordlabelsStr := strings.Trim(fmt.Sprint(requestedRecordLabels), "[]")
		log.Trace().Msgf("[%s] Requested record labels: %v", requestData.Indexer, recordlabelsStr)

		isRecordLabelPresent := false
		for _, rLabel := range requestedRecordLabels {
			if rLabel == recordLabel {
				isRecordLabelPresent = true
				break
			}
		}

		if !isRecordLabelPresent {
			log.Debug().Msgf("[%s] The record label '%s' is not included in the requested record labels: %v", requestData.Indexer, recordLabel, requestedRecordLabels)
			http.Error(w, "Record label not allowed", StatusLabelNotAllowed)
			return
		}
	}

	// hook size
	if requestData.TorrentID != 0 && (requestData.MinSize != 0 || requestData.MaxSize != 0) {
		if err := fetchTorrentDataIfNeeded(&requestData, &torrentData, apiBase); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		torrentSize := bytesize.ByteSize(torrentData.Response.Torrent.Size)

		minSize := bytesize.ByteSize(requestData.MinSize)
		maxSize := bytesize.ByteSize(requestData.MaxSize)

		log.Trace().Msgf("[%s] Torrent size: %s, Requested size range: %s - %s", requestData.Indexer, torrentSize, requestData.MinSize, requestData.MaxSize)

		if (requestData.MinSize != 0 && torrentSize < minSize) ||
			(requestData.MaxSize != 0 && torrentSize > maxSize) {
			log.Debug().Msgf("[%s] Torrent size %s is outside the requested size range: %s to %s", requestData.Indexer, torrentSize, minSize, maxSize)
			http.Error(w, "Torrent size is outside the requested size range", StatusSizeNotAllowed)
			return
		}
	}

	// hook ratio
	if requestData.MinRatio != 0 {
		if err := fetchUserDataIfNeeded(&requestData, &userData, apiBase); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		ratio := userData.Response.Stats.Ratio
		minRatio := requestData.MinRatio
		username := userData.Response.Username

		log.Trace().Msgf("[%s] MinRatio set to %.2f for %s", requestData.Indexer, minRatio, username)

		if ratio < minRatio {
			http.Error(w, "Returned ratio is below minimum requirement", StatusRatioNotAllowed)
			log.Debug().Msgf("[%s] Returned ratio %.2f is below minratio %.2f for %s", requestData.Indexer, ratio, minRatio, username)
			return

		}
	}

	w.WriteHeader(http.StatusOK) // HTTP status code 200
	log.Info().Msgf("[%s] Conditions met, responding with status 200", requestData.Indexer)
}