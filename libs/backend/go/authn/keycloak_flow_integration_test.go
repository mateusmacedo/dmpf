//go:build integration

package authn_test

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
)

const (
	portalClient    = "dmpf-portal"
	portalCallback  = "http://127.0.0.1/callback"
	portalLoggedOut = "http://127.0.0.1/logged-out"
	httpTimeout     = 15 * time.Second
	maxLoginSteps   = 10
)

type formKind string

const (
	formUsername     formKind = "username"
	formPassword     formKind = "password"
	formOrganization formKind = "organization"
	formOTP          formKind = "otp"
)

var (
	formTag       = regexp.MustCompile(`(?s)<form\b([^>]*)>(.*?)</form>`)
	fieldTag      = regexp.MustCompile(`(?s)<(?:input|button)\b([^>]*)>`)
	attributePair = regexp.MustCompile(`([\w.:-]+)="([^"]*)"`)
	errorMessage  = regexp.MustCompile(`id="(?:kc-error-message|input-error[\w-]*)"[^>]*>([^<]*)`)
	apiClient     = &http.Client{Timeout: httpTimeout}
)

func keycloakURL(t *testing.T) string {
	t.Helper()
	return strings.TrimSuffix(tb.Env(t, "KEYCLOAK_URL"), "/")
}

func keycloakIssuer(t *testing.T) string {
	t.Helper()
	return keycloakURL(t) + "/realms/dmpf"
}

type account struct {
	username string
	password string
}

func seedAccount(t *testing.T, username string) account {
	t.Helper()
	return account{username: username, password: tb.Env(t, "KEYCLOAK_SEED_PASSWORD")}
}

type loginForm struct {
	kind          formKind
	action        string
	hidden        url.Values
	visible       map[string]bool
	organizations []string
}

type step struct {
	callback url.Values
	form     *loginForm
	status   int
	message  string
}

func (s step) String() string {
	switch {
	case s.callback != nil:
		return "callback without code: " + s.callback.Encode()
	case s.form != nil:
		return fmt.Sprintf("login stopped at the %s form %v", s.form.kind, s.form.organizations)
	default:
		return fmt.Sprintf("HTTP %d page: %q", s.status, s.message)
	}
}

func (s step) code(t *testing.T) string {
	t.Helper()
	code := s.callback.Get("code")
	if code == "" {
		t.Fatalf("authorization ended without a code (%s): the login flow did not reach the redirect", s)
	}
	return code
}

type tokenSet struct {
	AccessToken      string `json:"access_token"`
	IDToken          string `json:"id_token"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

type browser struct {
	t        *testing.T
	issuer   string
	clientID string
	client   *http.Client
	verifier string
}

func newBrowser(t *testing.T) *browser {
	t.Helper()
	return browserFor(t, portalClient)
}

func browserFor(t *testing.T, clientID string) *browser {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New() err = %v", err)
	}
	return &browser{t: t, issuer: keycloakIssuer(t), clientID: clientID, client: &http.Client{
		Jar:           jar,
		Timeout:       httpTimeout,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}
}

func (b *browser) codeRequest(scope string) url.Values {
	b.verifier = rand.Text() + rand.Text()
	challenge := sha256.Sum256([]byte(b.verifier))
	return url.Values{
		"client_id":             {b.clientID},
		"response_type":         {"code"},
		"redirect_uri":          {portalCallback},
		"scope":                 {strings.TrimSpace("openid " + scope)},
		"state":                 {rand.Text()},
		"nonce":                 {rand.Text()},
		"code_challenge":        {base64.RawURLEncoding.EncodeToString(challenge[:])},
		"code_challenge_method": {"S256"},
	}
}

func (b *browser) authorize(params url.Values) step {
	b.t.Helper()
	return b.follow(b.send(http.MethodGet, b.issuer+"/protocol/openid-connect/auth?"+params.Encode(), nil))
}

func (b *browser) signIn(a account, scope string) (tokenSet, []formKind) {
	b.t.Helper()
	s, answered := b.login(a, b.authorize(b.codeRequest(scope)))
	return b.exchange(s.code(b.t)), answered
}

func (b *browser) login(a account, s step) (step, []formKind) {
	b.t.Helper()
	var answered []formKind
	for range maxLoginSteps {
		if s.form == nil {
			return s, answered
		}
		answer, ok := a.answer(s.form)
		if !ok {
			return s, answered
		}
		answered = append(answered, s.form.kind)
		s = b.submit(s.form, answer)
	}
	b.t.Fatalf("login of %s did not settle in %d forms: the flow is looping", a.username, maxLoginSteps)
	return step{}, nil
}

func (a account) answer(form *loginForm) (url.Values, bool) {
	switch form.kind {
	case formUsername:
		return url.Values{"username": {a.username}}, true
	case formPassword:
		answer := url.Values{"password": {a.password}}
		if form.visible["username"] {
			answer.Set("username", a.username)
		}
		return answer, true
	default:
		return nil, false
	}
}

func (b *browser) submit(form *loginForm, answer url.Values) step {
	b.t.Helper()
	values := url.Values{}
	for name, value := range form.hidden {
		values[name] = value
	}
	for name, value := range answer {
		values[name] = value
	}
	return b.follow(b.send(http.MethodPost, form.action, values))
}

func (b *browser) exchange(code string) tokenSet {
	b.t.Helper()
	status, tokens := requestToken(b.t, b.issuer, url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {b.clientID},
		"code":          {code},
		"redirect_uri":  {portalCallback},
		"code_verifier": {b.verifier},
	})
	if status != http.StatusOK || tokens.AccessToken == "" {
		b.t.Fatalf("code exchange = HTTP %d %s (%s): a code from the login must buy tokens with its verifier",
			status, tokens.Error, tokens.ErrorDescription)
	}
	return tokens
}

func (b *browser) logout(idToken string) {
	b.t.Helper()
	query := url.Values{"id_token_hint": {idToken}, "post_logout_redirect_uri": {portalLoggedOut}}
	response := b.send(http.MethodGet, b.issuer+"/protocol/openid-connect/logout?"+query.Encode(), nil)
	defer func() { _ = response.Body.Close() }()
	if location := response.Header.Get("Location"); response.StatusCode != http.StatusFound ||
		!strings.HasPrefix(location, portalLoggedOut) {
		b.t.Fatalf("logout = HTTP %d to %q: an id_token_hint with a registered redirect must end the session without a prompt",
			response.StatusCode, location)
	}
}

func (b *browser) send(method, target string, form url.Values) *http.Response {
	b.t.Helper()
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	request, err := http.NewRequestWithContext(b.t.Context(), method, target, body)
	if err != nil {
		b.t.Fatalf("http.NewRequest(%s) err = %v", method, err)
	}
	if form != nil {
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	response, err := b.client.Do(request)
	if err != nil {
		b.t.Fatalf("%s %s err = %v: Keycloak must answer the browser", method, request.URL.Path, err)
	}
	return response
}

func (b *browser) follow(response *http.Response) step {
	b.t.Helper()
	for range maxLoginSteps {
		location, err := response.Location()
		if err != nil {
			return b.page(response)
		}
		_, _ = io.Copy(io.Discard, response.Body)
		_ = response.Body.Close()
		if location.Host == "127.0.0.1" && strings.HasPrefix(location.String(), portalCallback) {
			params := location.Query()
			fragment, _ := url.ParseQuery(location.Fragment)
			for name, value := range fragment {
				params[name] = value
			}
			return step{callback: params}
		}
		response = b.send(http.MethodGet, location.String(), nil)
	}
	b.t.Fatalf("more than %d redirects: Keycloak is bouncing the browser", maxLoginSteps)
	return step{}
}

func (b *browser) page(response *http.Response) step {
	b.t.Helper()
	defer func() { _ = response.Body.Close() }()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		b.t.Fatalf("reading the login page err = %v", err)
	}
	s := step{status: response.StatusCode, form: parseLoginForm(string(body))}
	if match := errorMessage.FindStringSubmatch(string(body)); match != nil {
		s.message = strings.Join(strings.Fields(html.UnescapeString(match[1])), " ")
	}
	return s
}

func parseLoginForm(body string) *loginForm {
	for _, match := range formTag.FindAllStringSubmatch(body, -1) {
		attrs := attributes(match[1])
		form := &loginForm{action: attrs["action"], hidden: url.Values{}, visible: map[string]bool{}}
		for _, field := range fieldTag.FindAllStringSubmatch(match[2], -1) {
			fieldAttrs := attributes(field[1])
			switch name := fieldAttrs["name"]; {
			case name == "kc.org":
				form.organizations = append(form.organizations, fieldAttrs["value"])
			case name != "" && fieldAttrs["type"] == "hidden":
				form.hidden.Set(name, fieldAttrs["value"])
			case name != "":
				form.visible[name] = true
			}
		}
		if form.kind = kindOf(attrs["id"], form); form.kind != "" && form.action != "" {
			return form
		}
	}
	return nil
}

func kindOf(id string, form *loginForm) formKind {
	switch {
	case id == "kc-otp-login-form":
		return formOTP
	case len(form.organizations) > 0:
		return formOrganization
	case id == "kc-form-login" && form.visible["password"]:
		return formPassword
	case id == "kc-form-login" && form.visible["username"]:
		return formUsername
	default:
		return ""
	}
}

func attributes(tag string) map[string]string {
	attrs := map[string]string{}
	for _, match := range attributePair.FindAllStringSubmatch(tag, -1) {
		attrs[match[1]] = html.UnescapeString(match[2])
	}
	return attrs
}

func requestToken(t *testing.T, issuer string, form url.Values) (int, tokenSet) {
	t.Helper()
	response, err := apiClient.PostForm(issuer+"/protocol/openid-connect/token", form)
	if err != nil {
		t.Fatalf("token endpoint err = %v: Keycloak must answer the grant", err)
	}
	defer func() { _ = response.Body.Close() }()
	var tokens tokenSet
	if err := json.NewDecoder(response.Body).Decode(&tokens); err != nil {
		t.Fatalf("token endpoint HTTP %d answered no JSON: %v", response.StatusCode, err)
	}
	return response.StatusCode, tokens
}

func totp(key string, at time.Time) string {
	var counter [8]byte
	binary.BigEndian.PutUint64(counter[:], uint64(at.Unix()/30))
	mac := hmac.New(sha1.New, []byte(key))
	mac.Write(counter[:])
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	return fmt.Sprintf("%06d", (binary.BigEndian.Uint32(sum[offset:offset+4])&0x7fffffff)%1_000_000)
}

func tokenClaims(t *testing.T, token string) map[string]any {
	t.Helper()
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("token has %d segments, want a compact JWS", len(parts))
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("token payload is not base64url: %v", err)
	}
	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		t.Fatalf("token payload is not JSON: %v", err)
	}
	return claims
}

func claimText(claims map[string]any, name string) string {
	text, _ := claims[name].(string)
	return text
}

func requireTenant(t *testing.T, identity ports.Identity, want string) {
	t.Helper()
	switch {
	case want == "" && identity.Tenant != nil:
		t.Fatalf("Identity.Tenant = %q, want nil: without exactly one organization no tenant is resolved", *identity.Tenant)
	case want != "" && identity.Tenant == nil:
		t.Fatalf("Identity.Tenant = nil, want %q: the organization scope must carry its tenant_id", want)
	case want != "" && string(*identity.Tenant) != want:
		t.Fatalf("Identity.Tenant = %q, want %q", *identity.Tenant, want)
	}
}
