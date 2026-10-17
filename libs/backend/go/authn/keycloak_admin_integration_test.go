//go:build integration

package authn_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
)

const otpCredentialData = `{"subType":"totp","digits":6,"period":30,"algorithm":"HmacSHA1"}`

type clientFlows struct {
	ClientID    string `json:"clientId"`
	DirectGrant bool   `json:"directAccessGrantsEnabled"`
	Implicit    bool   `json:"implicitFlowEnabled"`
}

type adminAPI struct {
	t     *testing.T
	base  string
	token string
}

func masterAdmin(t *testing.T) *adminAPI {
	t.Helper()
	return adminAs(t, keycloakURL(t)+"/realms/master", url.Values{"grant_type": {"password"}, "client_id": {"admin-cli"},
		"username": {tb.Env(t, "KEYCLOAK_ADMIN_USER")}, "password": {tb.Env(t, "KEYCLOAK_ADMIN_PASSWORD")}})
}

func sessionEventsReader(t *testing.T) *adminAPI {
	t.Helper()
	return adminAs(t, keycloakIssuer(t), url.Values{"grant_type": {"client_credentials"}, "client_id": {"dmpf-session-events"},
		"client_secret": {tb.Env(t, "KEYCLOAK_SESSION_EVENTS_SECRET")}})
}

func adminAs(t *testing.T, issuer string, grant url.Values) *adminAPI {
	t.Helper()
	status, tokens := requestToken(t, issuer, grant)
	if status != http.StatusOK {
		t.Fatalf("%s token = HTTP %d %s: the client must authenticate to read the Admin REST",
			grant.Get("client_id"), status, tokens.Error)
	}
	return &adminAPI{t: t, base: keycloakURL(t) + "/admin/realms/dmpf", token: tokens.AccessToken}
}

func (a *adminAPI) call(method, path string, payload any, want int) ([]byte, http.Header) {
	a.t.Helper()
	encoded, err := json.Marshal(payload)
	if err != nil {
		a.t.Fatalf("json.Marshal(%s %s) err = %v", method, path, err)
	}
	var body io.Reader
	if payload != nil {
		body = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(context.Background(), method, a.base+path, body)
	if err != nil {
		a.t.Fatalf("http.NewRequest(%s %s) err = %v", method, path, err)
	}
	request.Header.Set("Authorization", "Bearer "+a.token)
	request.Header.Set("Content-Type", "application/json")
	response, err := apiClient.Do(request)
	if err != nil {
		a.t.Fatalf("%s %s err = %v: the Admin REST must answer", method, path, err)
	}
	defer func() { _ = response.Body.Close() }()
	answer, err := io.ReadAll(response.Body)
	if err != nil || response.StatusCode != want {
		a.t.Fatalf("%s %s = HTTP %d (%v), want %d: %s", method, path, response.StatusCode, err, want, answer)
	}
	return answer, response.Header
}

func (a *adminAPI) create(collection string, payload any) string {
	a.t.Helper()
	_, header := a.call(http.MethodPost, collection, payload, http.StatusCreated)
	return path.Base(header.Get("Location"))
}

func (a *adminAPI) decode(resource string, into any) {
	a.t.Helper()
	body, _ := a.call(http.MethodGet, resource, nil, http.StatusOK)
	if err := json.Unmarshal(body, into); err != nil {
		a.t.Fatalf("GET %s answered unexpected JSON (%v): %s", resource, err, body)
	}
}

func removeOnCleanup(t *testing.T, resource string) {
	t.Helper()
	// WHY: the admin-cli token of the master realm lives 60 s (Keycloak's default
	// accessTokenLifespan), so the token that created the resource may be expired here.
	t.Cleanup(func() { masterAdmin(t).call(http.MethodDelete, resource, nil, http.StatusNoContent) })
}

func disposablePortal(t *testing.T, backchannelURL string) string {
	t.Helper()
	clientID := "dmpf-it-portal-" + strings.ToLower(rand.Text())
	id := masterAdmin(t).create("/clients", map[string]any{
		"clientId":                  clientID,
		"protocol":                  "openid-connect",
		"publicClient":              true,
		"standardFlowEnabled":       true,
		"directAccessGrantsEnabled": false,
		"implicitFlowEnabled":       false,
		"frontchannelLogout":        false,
		"redirectUris":              []string{portalCallback},
		"attributes": map[string]string{
			"pkce.code.challenge.method":          "S256",
			"post.logout.redirect.uris":           portalLoggedOut,
			"backchannel.logout.url":              backchannelURL,
			"backchannel.logout.session.required": "true",
		},
		"defaultClientScopes":  []string{"web-origins", "acr", "roles", "profile", "basic", "email"},
		"optionalClientScopes": []string{"organization"},
		"protocolMappers": []map[string]any{{
			"name":           "dmpf-bff-audience",
			"protocol":       "openid-connect",
			"protocolMapper": "oidc-audience-mapper",
			"config": map[string]string{
				"included.client.audience": "dmpf-bff",
				"id.token.claim":           "false",
				"access.token.claim":       "true",
			},
		}},
	})
	removeOnCleanup(t, "/clients/"+id)
	return clientID
}

func disposableMFAUser(t *testing.T) (account, string) {
	t.Helper()
	admin := masterAdmin(t)
	user := account{username: "dmpf-it-mfa-" + strings.ToLower(rand.Text()), password: rand.Text()}
	otpKey := rand.Text()
	secretData, err := json.Marshal(map[string]string{"value": otpKey})
	if err != nil {
		t.Fatalf("json.Marshal(secretData) err = %v", err)
	}
	id := admin.create("/users", map[string]any{
		"username":      user.username,
		"enabled":       true,
		"emailVerified": true,
		"email":         user.username + "@dmpf.test",
		"firstName":     "Mfa",
		"lastName":      "Disposable",
		"credentials": []map[string]any{
			{"type": "password", "value": user.password, "temporary": false},
			{"type": "otp", "userLabel": "integration", "secretData": string(secretData), "credentialData": otpCredentialData},
		},
	})
	removeOnCleanup(t, "/users/"+id)

	var role json.RawMessage
	admin.decode("/roles/mfa-required", &role)
	admin.call(http.MethodPost, "/users/"+id+"/role-mappings/realm", []json.RawMessage{role}, http.StatusNoContent)
	admin.call(http.MethodPost, "/organizations/"+organizationID(admin, "acme")+"/members", id, http.StatusCreated)
	return user, otpKey
}

func organizationID(admin *adminAPI, alias string) string {
	admin.t.Helper()
	var organizations []struct {
		ID    string `json:"id"`
		Alias string `json:"alias"`
	}
	admin.decode("/organizations?"+url.Values{"search": {alias}}.Encode(), &organizations)
	for _, organization := range organizations {
		if organization.Alias == alias {
			return organization.ID
		}
	}
	admin.t.Fatalf("organization %s not found among %v: the seed creates it", alias, organizations)
	return ""
}
