package auth

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	telegramIssuer       = "https://oauth.telegram.org"
	authorizationURL     = "https://oauth.telegram.org/auth"
	tokenURL             = "https://oauth.telegram.org/token"
	jwksURL              = "https://oauth.telegram.org/.well-known/jwks.json"
	tokenRefreshInterval = 1 * time.Hour
)

// OIDCUser holds the claims extracted from a Telegram ID token.
type OIDCUser struct {
	ID            int64  `json:"id"`
	Subject       string `json:"sub"`
	Name          string `json:"name,omitempty"`
	GivenName     string `json:"given_name,omitempty"`
	FamilyName    string `json:"family_name,omitempty"`
	Username      string `json:"preferred_username,omitempty"`
	Picture       string `json:"picture,omitempty"`
	PhoneNumber   string `json:"phone_number,omitempty"`
	PhoneVerified bool   `json:"phone_number_verified"`
}

// OIDCClient performs the Telegram OIDC authorization code flow.
type OIDCClient struct {
	clientID     string
	clientSecret string
	redirectURI  string
	scopes       []string
	http         *http.Client

	jwksMutex   sync.RWMutex
	jwks        *JWKS
	jwksFetched time.Time
}

// NewOIDCClient creates a Telegram OIDC client.
func NewOIDCClient(clientID, clientSecret, redirectURI string, scopes []string) *OIDCClient {
	return NewOIDCClientWithHTTP(clientID, clientSecret, redirectURI, scopes, &http.Client{Timeout: 30 * time.Second})
}

// NewOIDCClientWithHTTP creates an OIDC client with a custom HTTP client (useful for tests).
func NewOIDCClientWithHTTP(clientID, clientSecret, redirectURI string, scopes []string, httpClient *http.Client) *OIDCClient {
	return &OIDCClient{
		clientID:     clientID,
		clientSecret: clientSecret,
		redirectURI:  redirectURI,
		scopes:       scopes,
		http:         httpClient,
	}
}

// PKCE holds a code verifier and its S256 challenge.
type PKCE struct {
	Verifier        string
	Challenge       string
	ChallengeMethod string
}

// GeneratePKCE creates a new PKCE pair using S256.
func GeneratePKCE() (*PKCE, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("generate pkce verifier: %w", err)
	}
	verifier := base64.RawURLEncoding.EncodeToString(b)
	h := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(h[:])
	return &PKCE{
		Verifier:        verifier,
		Challenge:       challenge,
		ChallengeMethod: "S256",
	}, nil
}

// GenerateState returns a cryptographically random state value.
func GenerateState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate state: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// AuthorizationURL builds the Telegram OIDC authorization URL.
func (c *OIDCClient) AuthorizationURL(state string, pkce *PKCE) string {
	u, _ := url.Parse(authorizationURL)
	q := u.Query()
	q.Set("client_id", c.clientID)
	q.Set("redirect_uri", c.redirectURI)
	q.Set("response_type", "code")
	q.Set("scope", strings.Join(c.scopes, " "))
	q.Set("state", state)
	q.Set("code_challenge", pkce.Challenge)
	q.Set("code_challenge_method", pkce.ChallengeMethod)
	u.RawQuery = q.Encode()
	return u.String()
}

// TokenResponse is the response from the Telegram token endpoint.
type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
	IDToken     string `json:"id_token"`
}

// ExchangeCode exchanges an authorization code for tokens.
func (c *OIDCClient) ExchangeCode(code, verifier string) (*TokenResponse, error) {
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("redirect_uri", c.redirectURI)
	data.Set("client_id", c.clientID)
	data.Set("code_verifier", verifier)

	req, err := http.NewRequest(http.MethodPost, tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Basic "+basicAuth(c.clientID, c.clientSecret))

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	var tokens TokenResponse
	if err := json.Unmarshal(body, &tokens); err != nil {
		return nil, fmt.Errorf("decode token response: %w", err)
	}
	return &tokens, nil
}

// ValidateIDToken verifies the ID token signature and claims.
func (c *OIDCClient) ValidateIDToken(token string) (*OIDCUser, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid id token format")
	}

	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("decode token header: %w", err)
	}
	var header struct {
		Alg string `json:"alg"`
		Kid string `json:"kid"`
	}
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return nil, fmt.Errorf("parse token header: %w", err)
	}

	jwks, err := c.getJWKS()
	if err != nil {
		return nil, fmt.Errorf("fetch jwks: %w", err)
	}

	var key *JWK
	for i := range jwks.Keys {
		if jwks.Keys[i].Kid == header.Kid {
			key = &jwks.Keys[i]
			break
		}
	}
	if key == nil {
		return nil, fmt.Errorf("no jwk found for kid %s", header.Kid)
	}
	if key.Alg != header.Alg {
		return nil, fmt.Errorf("jwk alg mismatch: header=%s key=%s", header.Alg, key.Alg)
	}

	if err := c.verifySignature(header.Alg, key, parts[0]+"."+parts[1], parts[2]); err != nil {
		return nil, fmt.Errorf("signature verification failed: %w", err)
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("decode token payload: %w", err)
	}

	claims, err := parseIDTokenClaims(payloadBytes)
	if err != nil {
		return nil, fmt.Errorf("parse token claims: %w", err)
	}

	if claims.Issuer != telegramIssuer {
		return nil, fmt.Errorf("invalid issuer: %s", claims.Issuer)
	}
	if claims.Audience != c.clientID {
		return nil, fmt.Errorf("invalid audience: got %s, want %s", claims.Audience, c.clientID)
	}
	if time.Now().Unix() > claims.Expiry {
		return nil, fmt.Errorf("id token expired")
	}

	return &OIDCUser{
		ID:            claims.ID,
		Subject:       claims.Subject,
		Name:          claims.Name,
		GivenName:     claims.GivenName,
		FamilyName:    claims.FamilyName,
		Username:      claims.Username,
		Picture:       claims.Picture,
		PhoneNumber:   claims.PhoneNumber,
		PhoneVerified: claims.PhoneVerified,
	}, nil
}

// idTokenClaims holds the parsed ID token payload.
type idTokenClaims struct {
	Issuer        string
	Audience      string
	Subject       string
	Expiry        int64
	ID            int64
	Name          string
	GivenName     string
	FamilyName    string
	Username      string
	Picture       string
	PhoneNumber   string
	PhoneVerified bool
}

// parseIDTokenClaims decodes the JWT payload using json.Number so Telegram's
// numeric claims sent as strings are handled gracefully.
func parseIDTokenClaims(payload []byte) (*idTokenClaims, error) {
	var raw map[string]json.RawMessage
	dec := json.NewDecoder(bytes.NewReader(payload))
	dec.UseNumber()
	if err := dec.Decode(&raw); err != nil {
		return nil, err
	}

	c := &idTokenClaims{}
	if v, ok := raw["iss"]; ok {
		json.Unmarshal(v, &c.Issuer)
	}
	if v, ok := raw["aud"]; ok {
		json.Unmarshal(v, &c.Audience)
	}
	if v, ok := raw["sub"]; ok {
		json.Unmarshal(v, &c.Subject)
	}
	if v, ok := raw["exp"]; ok {
		n, err := parseNumber(v)
		if err != nil {
			return nil, fmt.Errorf("exp: %w", err)
		}
		c.Expiry = n
	}
	if v, ok := raw["id"]; ok {
		n, err := parseNumber(v)
		if err != nil {
			return nil, fmt.Errorf("id: %w", err)
		}
		c.ID = n
	}
	if v, ok := raw["name"]; ok {
		json.Unmarshal(v, &c.Name)
	}
	if v, ok := raw["given_name"]; ok {
		json.Unmarshal(v, &c.GivenName)
	}
	if v, ok := raw["family_name"]; ok {
		json.Unmarshal(v, &c.FamilyName)
	}
	if v, ok := raw["preferred_username"]; ok {
		json.Unmarshal(v, &c.Username)
	}
	if v, ok := raw["picture"]; ok {
		json.Unmarshal(v, &c.Picture)
	}
	if v, ok := raw["phone_number"]; ok {
		json.Unmarshal(v, &c.PhoneNumber)
	}
	if v, ok := raw["phone_number_verified"]; ok {
		json.Unmarshal(v, &c.PhoneVerified)
	}
	return c, nil
}

func parseNumber(v json.RawMessage) (int64, error) {
	var s string
	if err := json.Unmarshal(v, &s); err == nil {
		return strconv.ParseInt(s, 10, 64)
	}
	var n json.Number
	if err := json.Unmarshal(v, &n); err != nil {
		return 0, err
	}
	return n.Int64()
}

func (c *OIDCClient) verifySignature(alg string, key *JWK, signingInput, signatureB64 string) error {
	signature, err := base64.RawURLEncoding.DecodeString(signatureB64)
	if err != nil {
		return fmt.Errorf("decode signature: %w", err)
	}

	switch alg {
	case "RS256":
		pubKey, err := key.RSAPublicKey()
		if err != nil {
			return fmt.Errorf("parse rsa key: %w", err)
		}
		hash := sha256.Sum256([]byte(signingInput))
		return rsa.VerifyPKCS1v15(pubKey, crypto.SHA256, hash[:], signature)
	case "ES256":
		return c.verifyES256(key, signingInput, signature)
	default:
		return fmt.Errorf("unsupported signing algorithm: %s", alg)
	}
}

func (c *OIDCClient) verifyES256(key *JWK, signingInput string, signature []byte) error {
	pubKey, err := key.ECPublicKey("P-256")
	if err != nil {
		return fmt.Errorf("parse ec key: %w", err)
	}
	hash := sha256.Sum256([]byte(signingInput))
	if len(signature) != 64 {
		return fmt.Errorf("invalid es256 signature length")
	}
	if !ecdsa.Verify(pubKey, hash[:], new(big.Int).SetBytes(signature[:32]), new(big.Int).SetBytes(signature[32:])) {
		return fmt.Errorf("invalid es256 signature")
	}
	return nil
}

func (c *OIDCClient) getJWKS() (*JWKS, error) {
	c.jwksMutex.RLock()
	cached := c.jwks
	fetched := c.jwksFetched
	c.jwksMutex.RUnlock()

	if cached != nil && time.Since(fetched) < tokenRefreshInterval {
		return cached, nil
	}

	resp, err := c.http.Get(jwksURL)
	if err != nil {
		if cached != nil {
			return cached, nil
		}
		return nil, fmt.Errorf("fetch jwks: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		if cached != nil {
			return cached, nil
		}
		return nil, fmt.Errorf("read jwks: %w", err)
	}

	var jwks JWKS
	if err := json.Unmarshal(body, &jwks); err != nil {
		if cached != nil {
			return cached, nil
		}
		return nil, fmt.Errorf("decode jwks: %w", err)
	}

	c.jwksMutex.Lock()
	c.jwks = &jwks
	c.jwksFetched = time.Now()
	c.jwksMutex.Unlock()

	return &jwks, nil
}

func basicAuth(username, password string) string {
	return base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
}

// JWKS represents a JSON Web Key Set.
type JWKS struct {
	Keys []JWK `json:"keys"`
}

// JWK represents a JSON Web Key.
type JWK struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Alg string `json:"alg"`
	Crv string `json:"crv,omitempty"`
	N   string `json:"n,omitempty"`
	E   string `json:"e,omitempty"`
	X   string `json:"x,omitempty"`
	Y   string `json:"y,omitempty"`
}

// RSAPublicKey parses the JWK into an RSA public key.
func (j *JWK) RSAPublicKey() (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(j.N)
	if err != nil {
		return nil, fmt.Errorf("decode modulus: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(j.E)
	if err != nil {
		return nil, fmt.Errorf("decode exponent: %w", err)
	}

	n := new(big.Int).SetBytes(nBytes)
	e := int(new(big.Int).SetBytes(eBytes).Int64())
	return &rsa.PublicKey{N: n, E: e}, nil
}

// ECPublicKey parses the JWK into an ECDSA public key for the given curve.
func (j *JWK) ECPublicKey(crv string) (*ecdsa.PublicKey, error) {
	var curve elliptic.Curve
	switch crv {
	case "P-256":
		curve = elliptic.P256()
	case "secp256k1":
		curve = secp256k1()
	default:
		return nil, fmt.Errorf("unsupported curve: %s", crv)
	}

	xBytes, err := base64.RawURLEncoding.DecodeString(j.X)
	if err != nil {
		return nil, fmt.Errorf("decode x: %w", err)
	}
	yBytes, err := base64.RawURLEncoding.DecodeString(j.Y)
	if err != nil {
		return nil, fmt.Errorf("decode y: %w", err)
	}

	x := new(big.Int).SetBytes(xBytes)
	y := new(big.Int).SetBytes(yBytes)
	return &ecdsa.PublicKey{Curve: curve, X: x, Y: y}, nil
}

func secp256k1() elliptic.Curve {
	// secp256k1 parameters for ES256K support.
	return &elliptic.CurveParams{
		Name:    "secp256k1",
		BitSize: 256,
		P:       fromHex("FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEFFFFFC2F"),
		N:       fromHex("FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEBAAEDCE6AF48A03BBFD25E8CD0364141"),
		B:       fromHex("0000000000000000000000000000000000000000000000000000000000000007"),
		Gx:      fromHex("79BE667EF9DCBBAC55A06295CE870B07029BFCDB2DCE28D959F2815B16F81798"),
		Gy:      fromHex("483ADA7726A3C4655DA4FBFC0E1108A8FD17B448A68554199C47D08FFB10D4B8"),
	}
}

func fromHex(s string) *big.Int {
	n, _ := new(big.Int).SetString(s, 16)
	return n
}
