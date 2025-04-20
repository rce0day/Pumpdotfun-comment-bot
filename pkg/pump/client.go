package pump

import (
    "encoding/json"
    "compress/gzip"
    "github.com/andybalholm/brotli"
    "fmt"
    "io"
    "bytes"
    "net/http"
    "net/http/cookiejar"
    "net/url"
    "time"
    "math/rand"
    "regexp"
    "github.com/gagliardetto/solana-go"
)

type SolID struct {
    PublicKey  string
    PrivateKey string
    Keypair    *solana.PrivateKey
}

type UserAgentGenerator struct {
    userAgent  string
    secChUa    string
}

type PumpClient struct {
    userAgent     string
    secChUa       string
    authenticated bool
    baseURL       string
    baseURLV2     string
    client        *http.Client
    baseHeaders   map[string]string
    authID        *SolID
    newUser       bool
}

var chromeVersions = []string{
    "122.0.0.0",
    "123.0.0.0",
    "124.0.0.0",
}

func generateWallet() *SolID {
    privateKey := solana.NewWallet().PrivateKey
    publicKey := privateKey.PublicKey().String()
    privateKeyBytes := privateKey.String()

    return &SolID{
        PublicKey:  publicKey,
        PrivateKey: privateKeyBytes,
        Keypair:    &privateKey,
    }
}

func NewUserAgentGenerator() *UserAgentGenerator {
    rand.Seed(time.Now().UnixNano())
    version := chromeVersions[rand.Intn(len(chromeVersions))]
    ua := fmt.Sprintf("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/%s Safari/537.36", version)
    generator := &UserAgentGenerator{
        userAgent: ua,
        secChUa:   secCha(ua),
    }
    return generator
}

func secCha(userAgent string) string {
    re := regexp.MustCompile(`Chrome/(\d+)`)
    matches := re.FindStringSubmatch(userAgent)
    if len(matches) < 2 {
        return ""
    }
    chromeVersion := matches[1]
    return fmt.Sprintf(`"Chromium";v="%s", "Google Chrome";v="%s", "Not?A_Brand";v="99"`, 
        chromeVersion, chromeVersion)
}

func NewPumpClient(authID string) (*PumpClient, error) {
    jar, err := cookiejar.New(nil)
    if err != nil {
        return nil, fmt.Errorf("failed to create cookie jar: %v", err)
    }
    uaGen := NewUserAgentGenerator()
    wallet := generateWallet()
    proxyUser := "username"
    proxyPass := "password"
    proxyURL, err := url.Parse(fmt.Sprintf("http://%s:%s@host:port", 
        proxyUser, proxyPass))
    if err != nil {
        return nil, fmt.Errorf("failed to parse proxy URL: %v", err)
    }
    
    transport := &http.Transport{
        Proxy: http.ProxyURL(proxyURL),
        DisableCompression: false,
    }
    client := &http.Client{
        Transport: transport,
        Jar:       jar,
    }
    
    headers := map[string]string{
        "sec-ch-ua-platform": "\"Windows\"",
        "user-agent":         uaGen.userAgent,
        "sec-ch-ua":         uaGen.secChUa,
        "content-type":      "application/json",
        "sec-ch-ua-mobile":  "?0",
        "accept":            "*/*",
        "origin":           "https://pump.fun",
        "sec-fetch-site":   "same-site",
        "sec-fetch-mode":   "cors",
        "sec-fetch-dest":   "empty",
        "referer":          "https://pump.fun/",
        "accept-encoding":  "gzip, deflate, br",
        "accept-language":  "en-US,en;q=0.9",
    }

    return &PumpClient{
        userAgent:     uaGen.userAgent,
        secChUa:       uaGen.secChUa,
        authenticated: false,
        baseURL:       "https://frontend-api.pump.fun",
        baseURLV2:     "https://client-proxy-server.pump.fun",
        client:        client,
        baseHeaders:   headers,
        authID:        wallet,
        newUser:       authID == "",
    }, nil
}

func (s *SolID) SignPumpLogin() (int64, string, error) {
    timestamp := time.Now().UnixMilli()
    message := fmt.Sprintf("Sign in to pump.fun: %d", timestamp)
    signature, err := s.SignMessage(message)
    if err != nil {
        return 0, "", fmt.Errorf("failed to sign message: %v", err)
    }
    return timestamp, signature, nil
}

func (s *SolID) SignMessage(message string) (string, error) {
    messageBytes := []byte(message)
    signature, err := s.Keypair.Sign(messageBytes)
    if err != nil {
        return "", fmt.Errorf("failed to sign message: %v", err)
    }
    return signature.String(), nil
}

func (p *PumpClient) doRequest(method, url string, body []byte) (*http.Response, error) {
    var bodyReader io.Reader
    if body != nil {
        bodyReader = bytes.NewBuffer(body)
    }
    req, err := http.NewRequest(method, url, bodyReader)
    if err != nil {
        return nil, err
    }
    if method == "POST" && body != nil {
        req.Header.Set("Content-Type", "application/json")
    }
    for key, value := range p.baseHeaders {
        req.Header.Set(key, value)
    }
    return p.client.Do(req)
}

func (p *PumpClient) gatherCookies() error {
    resp, err := p.doRequest("GET", "https://pump.fun/board", nil)
    if err != nil {
        return fmt.Errorf("failed to make request: %v", err)
    }
    defer resp.Body.Close()
    if !p.isOK(resp) {
        return fmt.Errorf("failed to gather initial cookies, status code: %d", resp.StatusCode)
    }
    return nil
}

func (p *PumpClient) executeWeb3Login(timestamp int64, signedMessage string) error {
    payload := struct {
        Address   string `json:"address"`
        Signature string `json:"signature"`
        Timestamp int64  `json:"timestamp"`
    }{
        Address:   p.authID.PublicKey,
        Signature: signedMessage,
        Timestamp: timestamp,
    }
    payloadBytes, err := json.Marshal(payload)
    if err != nil {
        return fmt.Errorf("failed to marshal payload: %v", err)
    }
    resp, err := p.doRequest("POST", 
        fmt.Sprintf("%s/auth/login", p.baseURL), 
        payloadBytes)
    if err != nil {
        return fmt.Errorf("failed to send login request: %v", err)
    }
    defer resp.Body.Close()
    if resp.StatusCode != 201 {
        return fmt.Errorf("failed to login using Solana message signing, status: %d", 
            resp.StatusCode)
    }
    return nil
}

func (p *PumpClient) ForThread() (string, error) {
    resp, err := p.doRequest("GET", 
        fmt.Sprintf("%s/token/generateTokenForThread", p.baseURL), 
        nil)
    if err != nil {
        return "", fmt.Errorf("failed to send token request: %v", err)
    }
    defer resp.Body.Close()
    if resp.StatusCode != 200 {
        return "", fmt.Errorf("failed to generate token for thread, status: %d", 
            resp.StatusCode)
    }

    var bodyBytes []byte
    var readErr error
    switch resp.Header.Get("Content-Encoding") {
    case "gzip":
        reader, err := gzip.NewReader(resp.Body)
        if err != nil {
            return "", fmt.Errorf("failed to create gzip reader: %v", err)
        }
        defer reader.Close()
        bodyBytes, readErr = io.ReadAll(reader)
    case "br":
        bodyBytes, readErr = io.ReadAll(brotli.NewReader(resp.Body))
    default:
        bodyBytes, readErr = io.ReadAll(resp.Body)
    }
    if readErr != nil {
        return "", fmt.Errorf("failed to read response body: %v", readErr)
    }

    var tokenResp struct {
        Token string `json:"token"`
    }
    if err := json.Unmarshal(bodyBytes, &tokenResp); err != nil {
        return "", fmt.Errorf("failed to decode token response: %v, body: %s", err, string(bodyBytes))
    }
    p.baseHeaders["x-aws-proxy-token"] = tokenResp.Token
    return tokenResp.Token, nil
}

func (p *PumpClient) PostComment(ca string, message string, fileUri string) error {
    headers := map[string]string{
        "accept":             "*/*",
        "accept-language":    "en-GB,en-US;q=0.9,en;q=0.8",
        "cache-control":      "no-cache",
        "content-type":       "application/json",
        "dnt":               "1",
        "origin":            "https://pump.fun",
        "pragma":            "no-cache",
        "priority":          "u=1, i",
        "referer":           "https://pump.fun/",
        "sec-ch-ua":         p.secChUa,
        "sec-ch-ua-mobile":  "?0",
        "sec-ch-ua-platform": "\"Windows\"",
        "sec-fetch-dest":    "empty",
        "sec-fetch-mode":    "cors",
        "sec-fetch-site":    "same-site",
        "user-agent":        p.userAgent,
        "x-aws-proxy-token": p.baseHeaders["x-aws-proxy-token"],
    }

    payload := map[string]string{
        "text": message,
        "mint": ca,
    }
    if fileUri != "" {
        payload["image"] = fileUri
    }

    payloadBytes, err := json.Marshal(payload)
    if err != nil {
        return fmt.Errorf("failed to marshal comment payload: %v", err)
    }

    req, err := http.NewRequest("POST", "https://client-proxy-server.pump.fun/comment", bytes.NewBuffer(payloadBytes))
    if err != nil {
        return fmt.Errorf("failed to create request: %v", err)
    }

    for key, value := range headers {
        req.Header.Set(key, value)
    }

    resp, err := p.client.Do(req)
    if err != nil {
        return fmt.Errorf("failed to send comment request: %v", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != 200 {
        return fmt.Errorf("failed to post comment, status: %d", resp.StatusCode)
    }
    return nil
}

func (p *PumpClient) LikeMessage(msgID string) error {
    if !p.authenticated {
        return fmt.Errorf("authentication required")
    }

    resp, err := p.doRequest("POST", 
        fmt.Sprintf("%s/likes/%s", p.baseURL, msgID), 
        nil)
    if err != nil {
        return fmt.Errorf("failed to send like request: %v", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != 201 {
        return fmt.Errorf("failed to like message, status: %d", resp.StatusCode)
    }

    return nil
}

func (p *PumpClient) Start() error {
    if err := p.gatherCookies(); err != nil {
        return fmt.Errorf("failed to gather cookies: %v", err)
    }

    timestamp, signature, err := p.authID.SignPumpLogin()
    if err != nil {
        return fmt.Errorf("failed to sign login message: %v", err)
    }

    if err := p.executeWeb3Login(timestamp, signature); err != nil {
        return fmt.Errorf("failed to execute web3 login: %v", err)
    }

    p.authenticated = true
    return nil
}

func (p *PumpClient) isOK(resp *http.Response) bool {
    return resp.StatusCode >= 200 && resp.StatusCode < 300
}

func RandomDelay(minSeconds, maxSeconds int) time.Duration {
    if minSeconds == maxSeconds {
        return time.Duration(minSeconds) * time.Second
    }
    delay := rand.Intn(maxSeconds-minSeconds+1) + minSeconds
    return time.Duration(delay) * time.Second
}