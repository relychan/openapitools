// Copyright 2026 RelyChan Pte. Ltd
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package proxyhandler

import (
	"net/http"
	"slices"
	"strings"

	"github.com/hasura/goenvconf"
	"github.com/pb33f/go-yaml"
	"github.com/pb33f/libopenapi/datamodel/high/base"
	highv3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/orderedmap"
	"github.com/relychan/gohttpc/authc/authscheme"
	"github.com/relychan/gohttpc/authc/basicauth"
	"github.com/relychan/gohttpc/authc/httpauth"
	"github.com/relychan/gohttpc/authc/oauth2scheme"
	"github.com/relychan/goutils/httperror"
	"github.com/relychan/goutils/httpheader"
	"github.com/relychan/openapitools/oaschema"
)

// OpenAPIAuthenticator manages security schemes from the openapi document.
type OpenAPIAuthenticator struct {
	securitySchemes   map[string]authscheme.HTTPClientAuthenticator
	security          []*base.SecurityRequirement
	operationSecurity [][]*base.SecurityRequirement
}

// NewOpenAPIv3Authenticator an authenticator from an OpenAPI v3 document.
func NewOpenAPIv3Authenticator(
	document *highv3.Document,
	getEnv goenvconf.GetEnvFunc,
) (*OpenAPIAuthenticator, error) {
	result := &OpenAPIAuthenticator{
		securitySchemes:   make(map[string]authscheme.HTTPClientAuthenticator),
		security:          document.Security,
		operationSecurity: collectOperationSecurity(document),
	}

	if document.Components == nil || document.Components.SecuritySchemes == nil {
		return result, nil
	}

	for iter := document.Components.SecuritySchemes.Oldest(); iter != nil; iter = iter.Next() {
		if iter.Value == nil {
			continue
		}

		key := iter.Key
		security := iter.Value

		authScheme, err := result.createAuthenticatorFromSecurityScheme(key, security, getEnv)
		if err != nil {
			return nil, httperror.HTTPError{
				Type:     "about:blank",
				Title:    "Invalid Security Scheme",
				Detail:   "Failed to create an authenticator from security scheme: " + err.Error(),
				Instance: "#/components/securitySchemes/" + key,
				Code:     "invalid-security-scheme",
			}
		}

		if authScheme != nil {
			result.securitySchemes[key] = authScheme
		}
	}

	return result, nil
}

func collectOperationSecurity(document *highv3.Document) [][]*base.SecurityRequirement {
	if document.Paths == nil || document.Paths.PathItems == nil {
		return nil
	}

	var result [][]*base.SecurityRequirement

	for pathItem := range document.Paths.PathItems.ValuesFromOldest() {
		if pathItem == nil {
			continue
		}

		for operation := range pathItem.GetOperations().ValuesFromOldest() {
			if operation != nil && len(operation.Security) > 0 {
				result = append(result, operation.Security)
			}
		}
	}

	return result
}

// GetAuthenticator finds a suitable authenticator by security requirements.
//
// The operationSecurity argument follows the OpenAPI semantics as represented by libopenapi:
//   - nil (the security field is absent): inherits the document-level security requirements.
//   - non-nil empty slice (explicit `security: []`): no authentication is required.
//
// Requirements are evaluated in order. The first requirement whose security schemes
// are all configured with credentials wins. A requirement containing multiple schemes
// (logical AND) returns a composite authenticator that applies every scheme.
// An empty requirement (`- {}`) marks the authentication as optional and is skipped;
// if no other requirement is satisfiable, nil is returned.
//
// No authenticator is returned if no requirement matches. The default authenticator of
// the HTTP client (settings.http.authentication) is still applied in that case,
// unless the operation explicitly disables security, see [OpenAPIAuthenticator.IsSecurityDisabled].
func (oaa *OpenAPIAuthenticator) GetAuthenticator(
	operationSecurity []*base.SecurityRequirement,
) authscheme.HTTPClientAuthenticator {
	if oaa == nil || len(oaa.securitySchemes) == 0 {
		return nil
	}

	security := operationSecurity
	if security == nil {
		security = oaa.security
	}

	for _, sec := range security {
		if sec == nil || sec.ContainsEmptyRequirement || sec.Requirements == nil ||
			sec.Requirements.Len() == 0 {
			// optional authentication
			continue
		}

		authenticators := make([]authscheme.HTTPClientAuthenticator, 0, sec.Requirements.Len())

		for name := range sec.Requirements.KeysFromOldest() {
			au, ok := oaa.securitySchemes[name]
			if !ok {
				authenticators = nil

				break
			}

			authenticators = append(authenticators, au)
		}

		switch len(authenticators) {
		case 0:
			continue
		case 1:
			return authenticators[0]
		default:
			return compositeAuthenticator(authenticators)
		}
	}

	return nil
}

// IsSecurityDisabled checks if the operation security explicitly disables authentication (`security: []`).
// It is safe to call with a nil receiver.
func (*OpenAPIAuthenticator) IsSecurityDisabled(
	operationSecurity []*base.SecurityRequirement,
) bool {
	return operationSecurity != nil && len(operationSecurity) == 0
}

// compositeAuthenticator applies multiple authenticators for AND security requirements.
// The inner authenticators are owned by [OpenAPIAuthenticator] so Close is a no-op.
type compositeAuthenticator []authscheme.HTTPClientAuthenticator

// Authenticate the credential into the incoming request.
func (ca compositeAuthenticator) Authenticate(
	req *http.Request,
	options ...authscheme.AuthenticateOption,
) error {
	for _, au := range ca {
		err := au.Authenticate(req, options...)
		if err != nil {
			return err
		}
	}

	return nil
}

// Close terminates internal processes before destroyed.
func (compositeAuthenticator) Close() error {
	return nil
}

// NoopAuthenticator is an authenticator that does nothing.
// It is used to override the default authenticator of the HTTP client.
type NoopAuthenticator struct{}

// Authenticate does nothing.
func (NoopAuthenticator) Authenticate(*http.Request, ...authscheme.AuthenticateOption) error {
	return nil
}

// Close does nothing.
func (NoopAuthenticator) Close() error {
	return nil
}

func (oaa *OpenAPIAuthenticator) createAuthenticatorFromSecurityScheme(
	key string,
	security *highv3.SecurityScheme,
	getEnv goenvconf.GetEnvFunc,
) (authscheme.HTTPClientAuthenticator, error) {
	securityType, err := oaschema.ParseSecuritySchemeType(security.Type)
	if err != nil {
		return nil, err
	}

	authOptions := &authscheme.HTTPClientAuthenticatorOptions{
		GetEnv: getEnv,
	}

	switch securityType {
	case oaschema.APIKeyScheme:
		if security.Extensions == nil {
			return nil, nil
		}

		rawCredentials, present := security.Extensions.Get(oaschema.XRelySecurityCredentials)
		if !present || rawCredentials == nil {
			return nil, nil
		}

		inLocation, err := authscheme.ParseAuthLocation(security.In)
		if err != nil {
			return nil, err
		}

		return newStaticAuthenticator(security, inLocation, rawCredentials, authOptions)
	case oaschema.HTTPAuthScheme:
		if security.Extensions == nil {
			return nil, nil
		}

		rawCredentials, present := security.Extensions.Get(oaschema.XRelySecurityCredentials)
		if !present || rawCredentials == nil {
			return nil, nil
		}

		switch security.Scheme {
		case "basic":
			var creds BasicCredentials

			err = rawCredentials.Decode(&creds)
			if err != nil {
				return nil, err
			}

			if creds.Username == nil && creds.Password == nil {
				return nil, nil
			}

			return basicauth.NewBasicCredential(&basicauth.BasicAuthConfig{
				Header:   security.Name,
				Username: creds.Username,
				Password: creds.Password,
			}, authOptions)
		default:
			return newStaticAuthenticator(
				security,
				authscheme.InHeader,
				rawCredentials,
				authOptions,
			)
		}
	case oaschema.OAuth2Scheme:
		return oaa.createOAuthAuthenticator(key, security, authOptions)
	case oaschema.OpenIDConnectScheme, oaschema.MutualTLSScheme:
		return nil, nil
	default:
		return nil, oaschema.ErrInvalidSecuritySchemeType
	}
}

func newStaticAuthenticator(
	security *highv3.SecurityScheme,
	inLocation authscheme.AuthLocation,
	rawCredentials *yaml.Node,
	authOptions *authscheme.HTTPClientAuthenticatorOptions,
) (authscheme.HTTPClientAuthenticator, error) {
	tokenLocation := authscheme.TokenLocation{
		In:     inLocation,
		Name:   security.Name,
		Scheme: security.Scheme,
	}

	if tokenLocation.Name == "" && tokenLocation.In == authscheme.InHeader {
		tokenLocation.Name = httpheader.Authorization
	}

	err := tokenLocation.Validate()
	if err != nil {
		return nil, err
	}

	var creds APIKeyCredentials

	err = rawCredentials.Decode(&creds)
	if err != nil {
		return nil, err
	}

	if creds.APIKey == nil {
		return nil, nil
	}

	return httpauth.NewHTTPCredential(&httpauth.HTTPAuthConfig{
		TokenLocation: tokenLocation,
		Value:         *creds.APIKey,
	}, authOptions)
}

func (oaa *OpenAPIAuthenticator) createOAuthAuthenticator( //nolint:cyclop
	key string,
	security *highv3.SecurityScheme,
	authOptions *authscheme.HTTPClientAuthenticatorOptions,
) (authscheme.HTTPClientAuthenticator, error) {
	if security.Flows == nil || security.Flows.ClientCredentials == nil ||
		security.Flows.ClientCredentials.Extensions == nil {
		return nil, nil
	}

	rawCredentials, present := security.Flows.ClientCredentials.Extensions.Get(
		oaschema.XRelySecurityCredentials,
	)
	if !present || rawCredentials == nil {
		return nil, nil
	}

	var creds OAuth2Credentials

	err := rawCredentials.Decode(&creds)
	if err != nil {
		return nil, err
	}

	if creds.ClientID == nil && creds.ClientSecret == nil {
		return nil, nil
	}

	if creds.ClientID == nil || creds.ClientSecret == nil {
		return nil, errOAuth2ClientCredentialsRequired
	}

	tokenLocation := &authscheme.TokenLocation{
		In:     authscheme.InHeader,
		Name:   httpheader.Authorization,
		Scheme: "bearer",
	}

	// optionally parse custom options in the security.
	if security.Name != "" {
		tokenLocation.Name = security.Name
	}

	if security.In != "" {
		locationIn, err := authscheme.ParseAuthLocation(strings.ToLower(security.In))
		if err != nil {
			return nil, err
		}

		tokenLocation.In = locationIn
	}

	if security.Scheme != "" {
		tokenLocation.Scheme = security.Scheme
	}

	tokenURL := goenvconf.EnvString{}

	if security.Flows.ClientCredentials.TokenUrl != "" {
		tokenURL.Value = &security.Flows.ClientCredentials.TokenUrl
	}

	tokenURLEnv, err := parseStringFromExtensions(
		security.Flows.ClientCredentials.Extensions,
		oaschema.XRelyOAuth2TokenURLEnv,
	)
	if err != nil {
		return nil, err
	}

	if tokenURLEnv != "" {
		tokenURL.Variable = &tokenURLEnv
	}

	if tokenURL.IsZero() {
		return nil, errOAuth2TokenURLRequired
	}

	refreshURL := &goenvconf.EnvString{}

	if security.Flows.ClientCredentials.RefreshUrl != "" {
		refreshURL.Value = &security.Flows.ClientCredentials.RefreshUrl
	}

	refreshTokenURLEnv, err := parseStringFromExtensions(
		security.Flows.ClientCredentials.Extensions,
		oaschema.XRelyOAuth2RefreshURLEnv,
	)
	if err != nil {
		return nil, err
	}

	if refreshTokenURLEnv != "" {
		refreshURL.Variable = &refreshTokenURLEnv
	}

	if refreshURL.IsZero() {
		refreshURL = nil
	}

	scopes := oaa.collectOAuth2Scopes(key)

	return oauth2scheme.NewOAuth2Credential(&oauth2scheme.OAuth2Config{
		TokenLocation: tokenLocation,
		Flows: oauth2scheme.OAuth2Flows{
			ClientCredentials: oauth2scheme.ClientCredentialsOAuthFlow{
				ClientID:       creds.ClientID,
				ClientSecret:   creds.ClientSecret,
				TokenURL:       &tokenURL,
				RefreshURL:     refreshURL,
				EndpointParams: creds.EndpointParams,
				Scopes:         scopes,
			},
		},
	}, authOptions)
}

// collectOAuth2Scopes collects requested scopes of the security scheme from
// document-level and operation-level security requirements.
// The authenticator is shared across operations so the token is requested with the union of scopes.
func (oaa *OpenAPIAuthenticator) collectOAuth2Scopes(key string) []string {
	scopes := []string{}

	appendScopes := func(security []*base.SecurityRequirement) {
		for _, sec := range security {
			if sec == nil || sec.ContainsEmptyRequirement || sec.Requirements == nil {
				continue
			}

			oauth2Sec, present := sec.Requirements.Get(key)
			if !present {
				continue
			}

			for _, scope := range oauth2Sec {
				if scope != "" && !slices.Contains(scopes, scope) {
					scopes = append(scopes, scope)
				}
			}
		}
	}

	appendScopes(oaa.security)

	for _, security := range oaa.operationSecurity {
		appendScopes(security)
	}

	return scopes
}

func parseStringFromExtensions(
	extensions *orderedmap.Map[string, *yaml.Node],
	key string,
) (string, error) {
	rawValue, present := extensions.Get(key)
	if !present || rawValue == nil {
		return "", nil
	}

	var envValue string

	err := rawValue.Decode(&envValue)

	return envValue, err
}
