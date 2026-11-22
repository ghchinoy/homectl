package roborock

import (
	"context"
	"crypto/hmac"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ghchinoy/homectl/modules/core"
)

const (
	// DefaultFileCloudDevices caches devices, local_keys, room names, and discovered LAN IPs
	// fetched from the user's Roborock account so vacuums remain paired to the official app
	// while homectl talks to them directly over LAN TCP (:58867).
	DefaultFileCloudDevices = "cloud_devices.json"

	defaultAgreementMajor = 14
	defaultAgreementMinor = 0
	maxCloudRespBytes     = 4 << 20 // 4 MiB safety limit
)

// DefaultCloudBaseURLs lists Roborock's regional Base IoT API servers in probe order.
var DefaultCloudBaseURLs = []string{
	"https://usiot.roborock.com",
	"https://euiot.roborock.com",
	"https://cniot.roborock.com",
	"https://ruiot.roborock.com",
}

// CloudLoginInfo stores the regional endpoint and country resolved for an email address.
type CloudLoginInfo struct {
	BaseURL     string `json:"base_url"`
	Country     string `json:"country"`
	CountryCode string `json:"country_code"`
}

// CloudReference holds the regional RRIoT URLs returned inside UserData.
type CloudReference struct {
	R string `json:"r,omitempty"`
	A string `json:"a,omitempty"`
	M string `json:"m,omitempty"`
	L string `json:"l,omitempty"`
}

// CloudRRiot holds the Hawk signing credentials returned on login.
type CloudRRiot struct {
	U string         `json:"u"`
	S string         `json:"s"`
	H string         `json:"h"`
	K string         `json:"k"`
	R CloudReference `json:"r"`
}

// CloudUserData holds the authenticated Roborock account session returned by LoginWithCode/LoginWithPassword.
type CloudUserData struct {
	UID         int64      `json:"uid,omitempty"`
	Token       string     `json:"token"`
	TokenType   string     `json:"tokentype,omitempty"`
	RRUID       string     `json:"rruid,omitempty"`
	Region      string     `json:"region,omitempty"`
	Country     string     `json:"country,omitempty"`
	CountryCode string     `json:"countrycode,omitempty"`
	Nickname    string     `json:"nickname,omitempty"`
	RRiot       CloudRRiot `json:"rriot"`
}

// CloudHomeProduct maps a Roborock product ID to its model string (e.g. roborock.vacuum.a144).
type CloudHomeProduct struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Model    string `json:"model"`
	Category string `json:"category,omitempty"`
	Code     string `json:"code,omitempty"`
}

// CloudHomeDevice describes a vacuum returned by /v3/user/homes/{id}.
type CloudHomeDevice struct {
	DUID      string `json:"duid"`
	Name      string `json:"name"`
	LocalKey  string `json:"localKey"`
	ProductID string `json:"productId"`
	FV        string `json:"fv,omitempty"`
	PV        string `json:"pv,omitempty"`
	SN        string `json:"sn,omitempty"`
	Online    bool   `json:"online,omitempty"`
}

// CloudHomeRoom maps a Roborock cloud room ID to its display name.
type CloudHomeRoom struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// CloudHomeData holds the devices, products, and rooms for a Roborock home.
type CloudHomeData struct {
	ID              int64              `json:"id"`
	Name            string             `json:"name"`
	Products        []CloudHomeProduct `json:"products"`
	Devices         []CloudHomeDevice  `json:"devices"`
	ReceivedDevices []CloudHomeDevice  `json:"receivedDevices"`
	Rooms           []CloudHomeRoom    `json:"rooms"`
}

// AllDevices returns owned and shared devices combined.
func (h *CloudHomeData) AllDevices() []CloudHomeDevice {
	if h == nil {
		return nil
	}
	out := make([]CloudHomeDevice, 0, len(h.Devices)+len(h.ReceivedDevices))
	out = append(out, h.Devices...)
	out = append(out, h.ReceivedDevices...)
	return out
}

// CloudClient communicates with Roborock's official Web API (v1/v3/v4) to perform a one-time
// account login and fetch each vacuum's DUID, local_key, model, and room names.
type CloudClient struct {
	Email      string
	BaseURLs   []string
	HTTPClient *http.Client

	deviceID  string
	loginInfo *CloudLoginInfo
}

// NewCloudClient constructs a Roborock cloud API client for email.
func NewCloudClient(email string, baseURL string) *CloudClient {
	urls := append([]string(nil), DefaultCloudBaseURLs...)
	if strings.TrimSpace(baseURL) != "" {
		urls = []string{strings.TrimRight(strings.TrimSpace(baseURL), "/")}
	}
	return &CloudClient{
		Email:      strings.TrimSpace(email),
		BaseURLs:   urls,
		HTTPClient: &http.Client{Timeout: 15 * time.Second},
		deviceID:   randomURLSafe(16),
	}
}

func (c *CloudClient) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return &http.Client{Timeout: 15 * time.Second}
}

func (c *CloudClient) headerClientID() string {
	h := md5.New()
	h.Write([]byte(c.Email))
	h.Write([]byte(c.deviceID))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func randomURLSafe(nBytes int) string {
	b := make([]byte, nBytes)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func randomAlphaNum(n int) string {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	_, _ = rand.Read(b)
	out := make([]byte, n)
	for i := range b {
		out[i] = chars[int(b[i])%len(chars)]
	}
	return string(out)
}

// SignHawkHeader generates the Hawk HMAC-SHA256 Authorization header required by RRIoT endpoints.
func SignHawkHeader(rriot CloudRRiot, reqPath string, params map[string]string, formData map[string]string, bodyJSON []byte, ts int64, nonce string) string {
	if ts <= 0 {
		ts = time.Now().Unix()
	}
	if nonce == "" {
		nonce = randomURLSafe(6)
	}
	paramsStr := hawkMapHash(params)
	var payloadStr string
	if bodyJSON != nil {
		sum := md5.Sum(bodyJSON)
		payloadStr = hex.EncodeToString(sum[:])
	} else {
		payloadStr = hawkMapHash(formData)
	}
	pathSum := md5.Sum([]byte(reqPath))
	prestr := strings.Join([]string{
		rriot.U,
		rriot.S,
		nonce,
		strconv.FormatInt(ts, 10),
		hex.EncodeToString(pathSum[:]),
		paramsStr,
		payloadStr,
	}, ":")

	mac := hmac.New(sha256.New, []byte(rriot.H))
	mac.Write([]byte(prestr))
	macB64 := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf(`Hawk id="%s",s="%s",ts="%d",nonce="%s",mac="%s"`, rriot.U, rriot.S, ts, nonce, macB64)
}

func hawkMapHash(values map[string]string) string {
	if len(values) == 0 {
		return ""
	}
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", k, values[k]))
	}
	sum := md5.Sum([]byte(strings.Join(parts, "&")))
	return hex.EncodeToString(sum[:])
}

func (c *CloudClient) doJSON(ctx context.Context, method, baseURL, reqPath string, params map[string]string, formData map[string]string, headers map[string]string) (map[string]any, error) {
	u, err := url.Parse(strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(reqPath, "/"))
	if err != nil {
		return nil, err
	}
	if len(params) > 0 {
		q := u.Query()
		for k, v := range params {
			q.Set(k, v)
		}
		u.RawQuery = q.Encode()
	}

	var bodyReader io.Reader
	if len(formData) > 0 {
		form := url.Values{}
		for k, v := range formData {
			form.Set(k, v)
		}
		bodyReader = strings.NewReader(form.Encode())
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), bodyReader)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if len(formData) > 0 && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("roborock cloud %s %s: %w", method, u.String(), err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxCloudRespBytes))
	if err != nil {
		return nil, fmt.Errorf("roborock cloud read body: %w", err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("roborock cloud HTTP %d invalid JSON: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	return out, nil
}

// GetURLByEmail resolves the user's regional Roborock API server and country metadata.
func (c *CloudClient) GetURLByEmail(ctx context.Context) (*CloudLoginInfo, error) {
	if c.loginInfo != nil {
		return c.loginInfo, nil
	}
	if c.Email == "" {
		return nil, errors.New("roborock cloud: email is required")
	}
	var lastErr error
	for _, base := range c.BaseURLs {
		resp, err := c.doJSON(ctx, http.MethodPost, base, "/api/v1/getUrlByEmail", map[string]string{
			"email":           c.Email,
			"needtwostepauth": "false",
		}, nil, nil)
		if err != nil {
			lastErr = err
			continue
		}
		code, _ := anyToInt(resp["code"])
		if code != 200 {
			if code == 2003 {
				return nil, fmt.Errorf("roborock cloud: invalid email format (%v)", resp["msg"])
			}
			lastErr = fmt.Errorf("roborock cloud getUrlByEmail code %d: %v", code, resp["msg"])
			continue
		}
		data, _ := resp["data"].(map[string]any)
		if data == nil {
			continue
		}
		resolvedURL := strings.TrimSpace(fmt.Sprint(data["url"]))
		country := strings.TrimSpace(stringFromAny(data["country"]))
		countryCode := strings.TrimSpace(stringFromAny(data["countrycode"]))
		if resolvedURL == "" || resolvedURL == "<nil>" {
			resolvedURL = base
		}
		if country != "" || countryCode != "" || len(c.BaseURLs) == 1 {
			c.loginInfo = &CloudLoginInfo{
				BaseURL:     strings.TrimRight(resolvedURL, "/"),
				Country:     country,
				CountryCode: countryCode,
			}
			return c.loginInfo, nil
		}
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, errors.New("roborock cloud: no regional server returned an account for this email")
}

// RequestEmailCode triggers a 6-digit verification code email using Roborock's v4 endpoint
// (falling back to v1 if country metadata is absent).
func (c *CloudClient) RequestEmailCode(ctx context.Context) error {
	info, err := c.GetURLByEmail(ctx)
	if err != nil {
		return err
	}
	if info.Country == "" || info.CountryCode == "" {
		return c.requestEmailCodeV1(ctx, info.BaseURL)
	}
	resp, err := c.doJSON(ctx, http.MethodPost, info.BaseURL, "/api/v4/email/code/send", nil, map[string]string{
		"email":    c.Email,
		"type":     "login",
		"platform": "",
	}, map[string]string{
		"header_clientid":   c.headerClientID(),
		"header_clientlang": "en",
		"Content-Type":      "application/x-www-form-urlencoded",
	})
	if err != nil {
		return err
	}
	code, _ := anyToInt(resp["code"])
	if code == 200 {
		return nil
	}
	switch code {
	case 2008:
		return errors.New("roborock cloud: account does not exist for this email")
	case 9002:
		return errors.New("roborock cloud: too many code requests — wait a few minutes and try again")
	case 3030:
		if len(c.BaseURLs) > 1 {
			c.BaseURLs = c.BaseURLs[1:]
			c.loginInfo = nil
			return c.RequestEmailCode(ctx)
		}
	}
	return fmt.Errorf("roborock cloud send code failed (code %d): %v", code, resp["msg"])
}

func (c *CloudClient) requestEmailCodeV1(ctx context.Context, baseURL string) error {
	resp, err := c.doJSON(ctx, http.MethodPost, baseURL, "/api/v1/sendEmailCode", map[string]string{
		"username": c.Email,
		"type":     "auth",
	}, nil, map[string]string{
		"header_clientid": c.headerClientID(),
	})
	if err != nil {
		return err
	}
	code, _ := anyToInt(resp["code"])
	if code != 200 {
		return fmt.Errorf("roborock cloud sendEmailCode failed (code %d): %v", code, resp["msg"])
	}
	return nil
}

func (c *CloudClient) signKeyV3(ctx context.Context, baseURL, s string) (string, error) {
	resp, err := c.doJSON(ctx, http.MethodPost, baseURL, "/api/v3/key/sign", map[string]string{"s": s}, nil, map[string]string{
		"header_clientid": c.headerClientID(),
	})
	if err != nil {
		return "", err
	}
	code, _ := anyToInt(resp["code"])
	if code != 200 {
		return "", fmt.Errorf("roborock cloud key/sign failed (code %d): %v", code, resp["msg"])
	}
	data, _ := resp["data"].(map[string]any)
	k := stringFromAny(data["k"])
	if k == "" {
		return "", errors.New("roborock cloud key/sign returned empty key")
	}
	return k, nil
}

func (c *CloudClient) getAgreementVersion(ctx context.Context, baseURL, country string) (int, int) {
	resp, err := c.doJSON(ctx, http.MethodGet, baseURL, "/api/v3/app/agreement/latest", map[string]string{
		"country": country,
	}, nil, map[string]string{
		"header_clientlang": "en",
	})
	if err != nil {
		return defaultAgreementMajor, defaultAgreementMinor
	}
	code, _ := anyToInt(resp["code"])
	if code != 200 {
		return defaultAgreementMajor, defaultAgreementMinor
	}
	data, _ := resp["data"].(map[string]any)
	major, ok1 := anyToInt(data["majorVersion"])
	minor, ok2 := anyToInt(data["minorVersion"])
	if ok1 && ok2 {
		return major, minor
	}
	return defaultAgreementMajor, defaultAgreementMinor
}

// LoginWithCode completes the v4 email-code login flow and returns the authenticated UserData.
func (c *CloudClient) LoginWithCode(ctx context.Context, verifyCode string) (*CloudUserData, error) {
	verifyCode = strings.TrimSpace(verifyCode)
	if verifyCode == "" {
		return nil, errors.New("roborock cloud: verification code is required")
	}
	info, err := c.GetURLByEmail(ctx)
	if err != nil {
		return nil, err
	}
	if info.Country == "" || info.CountryCode == "" {
		return c.loginWithCodeV1(ctx, info.BaseURL, verifyCode)
	}

	ks := randomAlphaNum(16)
	k, err := c.signKeyV3(ctx, info.BaseURL, ks)
	if err != nil {
		return nil, err
	}
	major, minor := c.getAgreementVersion(ctx, info.BaseURL, info.Country)

	resp, err := c.doJSON(ctx, http.MethodPost, info.BaseURL, "/api/v4/auth/email/login/code", nil, map[string]string{
		"country":      info.Country,
		"countryCode":  info.CountryCode,
		"email":        c.Email,
		"code":         verifyCode,
		"majorVersion": strconv.Itoa(major),
		"minorVersion": strconv.Itoa(minor),
	}, map[string]string{
		"header_clientid":    c.headerClientID(),
		"x-mercy-ks":         ks,
		"x-mercy-k":          k,
		"Content-Type":       "application/x-www-form-urlencoded",
		"header_clientlang":  "en",
		"header_appversion":  "4.54.02",
		"header_phonesystem": "iOS",
		"header_phonemodel":  "iPhone16,1",
	})
	if err != nil {
		return nil, err
	}
	return parseLoginUserData(resp)
}

func (c *CloudClient) loginWithCodeV1(ctx context.Context, baseURL, verifyCode string) (*CloudUserData, error) {
	resp, err := c.doJSON(ctx, http.MethodPost, baseURL, "/api/v1/loginWithCode", map[string]string{
		"username":       c.Email,
		"verifycode":     verifyCode,
		"verifycodetype": "AUTH_EMAIL_CODE",
	}, nil, map[string]string{
		"header_clientid": c.headerClientID(),
	})
	if err != nil {
		return nil, err
	}
	return parseLoginUserData(resp)
}

// LoginWithPassword authenticates via Roborock's legacy v1 email/password endpoint.
func (c *CloudClient) LoginWithPassword(ctx context.Context, password string) (*CloudUserData, error) {
	info, err := c.GetURLByEmail(ctx)
	if err != nil {
		return nil, err
	}
	resp, err := c.doJSON(ctx, http.MethodPost, info.BaseURL, "/api/v1/login", map[string]string{
		"username":        c.Email,
		"password":        password,
		"needtwostepauth": "false",
	}, nil, map[string]string{
		"header_clientid": c.headerClientID(),
	})
	if err != nil {
		return nil, err
	}
	return parseLoginUserData(resp)
}

func parseLoginUserData(resp map[string]any) (*CloudUserData, error) {
	code, _ := anyToInt(resp["code"])
	if code != 200 {
		switch code {
		case 2018:
			return nil, errors.New("roborock cloud: invalid verification code")
		case 3006:
			return nil, errors.New("roborock cloud: user agreement version rejected — accept agreement in the Roborock app and retry")
		case 3009:
			return nil, errors.New("roborock cloud: user agreement not accepted in Roborock app")
		case 3039:
			return nil, errors.New("roborock cloud: account does not exist in this region")
		default:
			return nil, fmt.Errorf("roborock cloud login failed (code %d): %v", code, resp["msg"])
		}
	}
	raw, err := json.Marshal(resp["data"])
	if err != nil {
		return nil, err
	}
	var ud CloudUserData
	if err := json.Unmarshal(raw, &ud); err != nil {
		return nil, fmt.Errorf("roborock cloud: parse user_data: %w", err)
	}
	if ud.Token == "" || ud.RRiot.R.A == "" {
		return nil, errors.New("roborock cloud: login succeeded but user_data is missing token or rriot API base")
	}
	return &ud, nil
}

// GetHomeData fetches the user's home inventory (devices + local_keys + models + rooms)
// using /v3/user/homes/{homeId} (with automatic fallback to /v2 and /).
func (c *CloudClient) GetHomeData(ctx context.Context, userData *CloudUserData) (*CloudHomeData, error) {
	if userData == nil || userData.Token == "" || userData.RRiot.R.A == "" {
		return nil, errors.New("roborock cloud: valid userData is required")
	}
	info, err := c.GetURLByEmail(ctx)
	if err != nil {
		return nil, err
	}
	homeDetail, err := c.doJSON(ctx, http.MethodGet, info.BaseURL, "/api/v1/getHomeDetail", nil, nil, map[string]string{
		"header_clientid": c.headerClientID(),
		"Authorization":   userData.Token,
	})
	if err != nil {
		return nil, err
	}
	code, _ := anyToInt(homeDetail["code"])
	if code != 200 {
		return nil, fmt.Errorf("roborock cloud getHomeDetail failed (code %d): %v", code, homeDetail["msg"])
	}
	detailData, _ := homeDetail["data"].(map[string]any)
	homeID, ok := anyToInt(detailData["rrHomeId"])
	if !ok || homeID <= 0 {
		return nil, errors.New("roborock cloud: getHomeDetail returned missing rrHomeId")
	}

	var lastErr error
	for _, prefix := range []string{"/v3", "/v2", ""} {
		reqPath := fmt.Sprintf("%s/user/homes/%d", prefix, homeID)
		auth := SignHawkHeader(userData.RRiot, reqPath, nil, nil, nil, 0, "")
		resp, err := c.doJSON(ctx, http.MethodGet, userData.RRiot.R.A, reqPath, nil, nil, map[string]string{
			"Authorization": auth,
		})
		if err != nil {
			lastErr = err
			continue
		}
		if success, _ := resp["success"].(bool); !success {
			lastErr = fmt.Errorf("roborock cloud %s failed: %v", reqPath, resp)
			continue
		}
		raw, err := json.Marshal(resp["result"])
		if err != nil {
			lastErr = err
			continue
		}
		var hd CloudHomeData
		if err := json.Unmarshal(raw, &hd); err != nil {
			lastErr = err
			continue
		}
		return &hd, nil
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, errors.New("roborock cloud: unable to fetch home data")
}

func stringFromAny(v any) string {
	if v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	case float64:
		return strconv.FormatInt(int64(x), 10)
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	default:
		return fmt.Sprint(x)
	}
}

// CloudDeviceRecord persists one vacuum's cloud-fetched local_key + metadata and cached LAN IP.
type CloudDeviceRecord struct {
	DUID            string `json:"duid"`
	Name            string `json:"name"`
	Model           string `json:"model"`
	ProductID       string `json:"product_id,omitempty"`
	LocalKey        string `json:"local_key"`
	FirmwareVersion string `json:"firmware_version,omitempty"`
	PV              string `json:"pv,omitempty"`
	SN              string `json:"sn,omitempty"`
	IP              string `json:"ip,omitempty"`
	LocalProtocol   string `json:"local_protocol,omitempty"` // "1.0" or "L01"
	LastDiscovered  string `json:"last_discovered,omitempty"`
}

// CloudAccountCache is the on-disk schema of cloud_devices.json (0600).
type CloudAccountCache struct {
	Email     string              `json:"email,omitempty"`
	Region    string              `json:"region,omitempty"`
	HomeID    int64               `json:"home_id,omitempty"`
	HomeName  string              `json:"home_name,omitempty"`
	UpdatedAt string              `json:"updated_at,omitempty"`
	Rooms     map[int]string      `json:"rooms,omitempty"`
	Devices   []CloudDeviceRecord `json:"devices"`
}

// CloudDeviceStore manages cloud_devices.json for keep-official-app LAN control.
type CloudDeviceStore struct {
	mu       sync.RWMutex
	storage  core.Storage
	filename string
	cache    CloudAccountCache
}

// NewCloudDeviceStore loads (or initializes empty) cloud_devices.json from storage.
func NewCloudDeviceStore(storage core.Storage) (*CloudDeviceStore, error) {
	if storage == nil {
		storage = core.NewMemoryStorage()
	}
	s := &CloudDeviceStore{
		storage:  storage,
		filename: DefaultFileCloudDevices,
		cache: CloudAccountCache{
			Rooms:   make(map[int]string),
			Devices: []CloudDeviceRecord{},
		},
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

// Reload re-reads cloud_devices.json from storage if it exists.
func (s *CloudDeviceStore) Reload() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.load()
}

func (s *CloudDeviceStore) load() error {
	raw, err := s.storage.ReadFile(s.filename)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("roborock: read %s: %w", s.filename, err)
	}
	var parsed CloudAccountCache
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return fmt.Errorf("roborock: parse %s: %w", s.filename, err)
	}
	if parsed.Rooms == nil {
		parsed.Rooms = make(map[int]string)
	}
	if parsed.Devices == nil {
		parsed.Devices = []CloudDeviceRecord{}
	}
	s.cache = parsed
	return nil
}

func (s *CloudDeviceStore) saveLocked() error {
	data, err := json.MarshalIndent(s.cache, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(s.storage, s.filename, data, 0600)
}

// SaveHomeData merges devices and room names from a CloudHomeData response, preserving any cached LAN IPs.
func (s *CloudDeviceStore) SaveHomeData(email, region string, hd *CloudHomeData) ([]CloudDeviceRecord, error) {
	if hd == nil {
		return nil, errors.New("roborock: home data is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	existingByDUID := make(map[string]CloudDeviceRecord, len(s.cache.Devices))
	for _, d := range s.cache.Devices {
		existingByDUID[d.DUID] = d
	}

	products := make(map[string]CloudHomeProduct, len(hd.Products))
	for _, p := range hd.Products {
		products[p.ID] = p
	}

	rooms := make(map[int]string, len(hd.Rooms))
	for _, r := range hd.Rooms {
		if r.ID > 0 && strings.TrimSpace(r.Name) != "" {
			rooms[r.ID] = strings.TrimSpace(r.Name)
		}
	}

	records := make([]CloudDeviceRecord, 0, len(hd.Devices)+len(hd.ReceivedDevices))
	for _, d := range hd.AllDevices() {
		if strings.TrimSpace(d.DUID) == "" || strings.TrimSpace(d.LocalKey) == "" {
			continue
		}
		model := ""
		if p, ok := products[d.ProductID]; ok {
			model = p.Model
		}
		rec := CloudDeviceRecord{
			DUID:            strings.TrimSpace(d.DUID),
			Name:            strings.TrimSpace(d.Name),
			Model:           strings.TrimSpace(model),
			ProductID:       strings.TrimSpace(d.ProductID),
			LocalKey:        strings.TrimSpace(d.LocalKey),
			FirmwareVersion: strings.TrimSpace(d.FV),
			PV:              strings.TrimSpace(d.PV),
			SN:              strings.TrimSpace(d.SN),
		}
		if prev, ok := existingByDUID[rec.DUID]; ok {
			rec.IP = prev.IP
			rec.LocalProtocol = prev.LocalProtocol
			rec.LastDiscovered = prev.LastDiscovered
		}
		records = append(records, rec)
	}
	sort.Slice(records, func(i, j int) bool {
		return records[i].Name < records[j].Name
	})

	s.cache = CloudAccountCache{
		Email:     strings.TrimSpace(email),
		Region:    strings.TrimSpace(region),
		HomeID:    hd.ID,
		HomeName:  strings.TrimSpace(hd.Name),
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
		Rooms:     rooms,
		Devices:   records,
	}
	if err := s.saveLocked(); err != nil {
		return nil, err
	}
	out := make([]CloudDeviceRecord, len(records))
	copy(out, records)
	return out, nil
}

// UpdateDiscovered updates cached LAN IPs and local protocol versions from UDP :58866 discovery.
func (s *CloudDeviceStore) UpdateDiscovered(found []BroadcastDevice) (int, error) {
	if len(found) == 0 {
		return 0, nil
	}
	byDUID := make(map[string]BroadcastDevice, len(found))
	for _, b := range found {
		if b.DUID != "" && b.IP != "" {
			byDUID[b.DUID] = b
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	updated := 0
	now := time.Now().UTC().Format(time.RFC3339)
	for i := range s.cache.Devices {
		b, ok := byDUID[s.cache.Devices[i].DUID]
		if !ok {
			continue
		}
		s.cache.Devices[i].IP = b.IP
		if b.Version != "" {
			s.cache.Devices[i].LocalProtocol = b.Version
		}
		s.cache.Devices[i].LastDiscovered = now
		updated++
	}
	if updated > 0 {
		if err := s.saveLocked(); err != nil {
			return updated, err
		}
	}
	return updated, nil
}

// Devices returns a copy of all cached cloud devices.
func (s *CloudDeviceStore) Devices() []CloudDeviceRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]CloudDeviceRecord, len(s.cache.Devices))
	copy(out, s.cache.Devices)
	return out
}

// Rooms returns a copy of the cloud room ID -> name map.
func (s *CloudDeviceStore) Rooms() map[int]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[int]string, len(s.cache.Rooms))
	for k, v := range s.cache.Rooms {
		out[k] = v
	}
	return out
}

// Find resolves a device by exact DUID, exact case-insensitive name, or unique case-insensitive name/model substring.
func (s *CloudDeviceStore) Find(query string) (*CloudDeviceRecord, bool) {
	q := strings.TrimSpace(query)
	if q == "" {
		return nil, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	lower := strings.ToLower(q)
	for _, d := range s.cache.Devices {
		if d.DUID == q || strings.EqualFold(d.Name, q) {
			cp := d
			return &cp, true
		}
	}
	var match *CloudDeviceRecord
	for _, d := range s.cache.Devices {
		if strings.Contains(strings.ToLower(d.Name), lower) || strings.Contains(strings.ToLower(d.Model), lower) {
			if match != nil {
				return nil, false // ambiguous
			}
			cp := d
			match = &cp
		}
	}
	return match, match != nil
}
