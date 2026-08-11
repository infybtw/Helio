package auth

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestGeneratePKCE(t *testing.T) {
	pkce, err := GeneratePKCE()
	if err != nil {
		t.Fatalf("generate pkce: %v", err)
	}
	if pkce.Verifier == "" || pkce.Challenge == "" {
		t.Fatal("expected non-empty verifier and challenge")
	}
	if pkce.ChallengeMethod != "S256" {
		t.Fatalf("expected S256, got %s", pkce.ChallengeMethod)
	}
	h := sha256.Sum256([]byte(pkce.Verifier))
	expected := base64.RawURLEncoding.EncodeToString(h[:])
	if pkce.Challenge != expected {
		t.Fatal("challenge does not match verifier hash")
	}
}

func TestAuthorizationURL(t *testing.T) {
	client := NewOIDCClient("123", "secret", "https://example.com/callback", []string{"openid", "profile"})
	pkce, _ := GeneratePKCE()
	url := client.AuthorizationURL("state123", pkce)

	if !strings.Contains(url, "client_id=123") {
		t.Error("missing client_id")
	}
	if !strings.Contains(url, "redirect_uri=https%3A%2F%2Fexample.com%2Fcallback") {
		t.Error("missing redirect_uri")
	}
	if !strings.Contains(url, "state=state123") {
		t.Error("missing state")
	}
	if !strings.Contains(url, "code_challenge="+pkce.Challenge) {
		t.Error("missing code_challenge")
	}
}

func TestValidateIDToken(t *testing.T) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}

	pub := &privKey.PublicKey
	n := base64.RawURLEncoding.EncodeToString(pub.N.Bytes())
	e := base64.RawURLEncoding.EncodeToString([]byte{1, 0, 1}) // 65537

	jwks := JWKS{
		Keys: []JWK{{
			Kty: "RSA",
			Kid: "test-key",
			Alg: "RS256",
			N:   n,
			E:   e,
		}},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jwks)
	}))
	defer server.Close()

	// Temporarily override the JWKS URL by using a custom transport that intercepts the known URL.
	httpClient := &http.Client{
		Transport: &rewriteTransport{
			base:     http.DefaultTransport,
			jwksURL:  server.URL + "/jwks",
			tokenURL: server.URL + "/token",
			authURL:  server.URL + "/auth",
		},
	}

	client := NewOIDCClientWithHTTP("client-id", "secret", "https://example.com/callback", []string{"openid", "profile"}, httpClient)

	now := time.Now().Unix()
	claims := map[string]any{
		"iss":                telegramIssuer,
		"aud":                "client-id",
		"sub":                "1234567890",
		"iat":                now,
		"exp":                now + 3600,
		"id":                 987654321,
		"name":               "John Doe",
		"given_name":         "John",
		"family_name":        "Doe",
		"preferred_username": "johndoe",
	}

	token := signTestJWT(t, privKey, "RS256", "test-key", claims)

	user, err := client.ValidateIDToken(token)
	if err != nil {
		t.Fatalf("validate id token: %v", err)
	}
	if user.ID != 987654321 {
		t.Errorf("expected id 987654321, got %d", user.ID)
	}
	if user.Username != "johndoe" {
		t.Errorf("expected username johndoe, got %s", user.Username)
	}
}

func TestValidateIDTokenInvalidIssuer(t *testing.T) {
	privKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	claims := map[string]any{
		"iss": "https://evil.com",
		"aud": "client-id",
		"sub": "1",
		"exp": time.Now().Unix() + 3600,
	}
	token := signTestJWT(t, privKey, "RS256", "k", claims)

	client := NewOIDCClient("client-id", "secret", "https://example.com/callback", []string{"openid"})
	if _, err := client.ValidateIDToken(token); err == nil {
		t.Fatal("expected error for invalid issuer")
	}
}

func TestValidateIDTokenStringID(t *testing.T) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}

	pub := &privKey.PublicKey
	n := base64.RawURLEncoding.EncodeToString(pub.N.Bytes())
	e := base64.RawURLEncoding.EncodeToString([]byte{1, 0, 1})

	jwks := JWKS{
		Keys: []JWK{{
			Kty: "RSA",
			Kid: "test-key",
			Alg: "RS256",
			N:   n,
			E:   e,
		}},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jwks)
	}))
	defer server.Close()

	httpClient := &http.Client{
		Transport: &rewriteTransport{
			base:    http.DefaultTransport,
			jwksURL: server.URL + "/jwks",
		},
	}

	client := NewOIDCClientWithHTTP("client-id", "secret", "https://example.com/callback", []string{"openid", "profile"}, httpClient)

	now := time.Now().Unix()
	claims := map[string]any{
		"iss":                telegramIssuer,
		"aud":                "client-id",
		"sub":                "1234567890",
		"iat":                now,
		"exp":                now + 3600,
		"id":                 "987654321",
		"name":               "John Doe",
		"given_name":         "John",
		"family_name":        "Doe",
		"preferred_username": "johndoe",
	}

	token := signTestJWT(t, privKey, "RS256", "test-key", claims)

	user, err := client.ValidateIDToken(token)
	if err != nil {
		t.Fatalf("validate id token with string id: %v", err)
	}
	if user.ID != 987654321 {
		t.Errorf("expected id 987654321, got %d", user.ID)
	}
}

func TestVerifyES256(t *testing.T) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate ES256 key: %v", err)
	}
	signingInput := "header.payload"
	hash := sha256.Sum256([]byte(signingInput))
	r, s, err := ecdsa.Sign(rand.Reader, privateKey, hash[:])
	if err != nil {
		t.Fatalf("sign ES256 payload: %v", err)
	}
	signature := make([]byte, 64)
	r.FillBytes(signature[:32])
	s.FillBytes(signature[32:])
	key := &JWK{
		Kty: "EC",
		Crv: "P-256",
		X:   base64.RawURLEncoding.EncodeToString(privateKey.X.FillBytes(make([]byte, 32))),
		Y:   base64.RawURLEncoding.EncodeToString(privateKey.Y.FillBytes(make([]byte, 32))),
	}

	if err := (&OIDCClient{}).verifyES256(key, signingInput, signature); err != nil {
		t.Fatalf("verify ES256 signature: %v", err)
	}
	if err := (&OIDCClient{}).verifyES256(key, signingInput, signature[:63]); err == nil {
		t.Fatal("expected invalid ES256 signature length error")
	}
}

func signTestJWT(t *testing.T, privKey *rsa.PrivateKey, alg, kid string, claims map[string]any) string {
	t.Helper()
	header := base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf(`{"alg":"%s","typ":"JWT","kid":"%s"}`, alg, kid)))

	payloadBytes, _ := json.Marshal(claims)
	payload := base64.RawURLEncoding.EncodeToString(payloadBytes)

	signingInput := header + "." + payload
	hash := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, privKey, crypto.SHA256, hash[:])
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	signature := base64.RawURLEncoding.EncodeToString(sig)
	return signingInput + "." + signature
}

type rewriteTransport struct {
	base     http.RoundTripper
	jwksURL  string
	tokenURL string
	authURL  string
}

func (rt *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.String() == jwksURL {
		newReq, _ := http.NewRequest(req.Method, rt.jwksURL, req.Body)
		newReq.Header = req.Header
		return rt.base.RoundTrip(newReq)
	}
	if req.URL.String() == tokenURL {
		newReq, _ := http.NewRequest(req.Method, rt.tokenURL, req.Body)
		newReq.Header = req.Header
		return rt.base.RoundTrip(newReq)
	}
	return rt.base.RoundTrip(req)
}
