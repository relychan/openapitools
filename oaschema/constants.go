package oaschema

const (
	// XRelyServerWeight is the extension name enum for the weight of server if the load balancer is configured.
	XRelyServerWeight = "x-rely-server-weight"
	// XRelyServerHeaders is the extension name enum for custom headers for the server.
	XRelyServerHeaders = "x-rely-server-headers"
	// XRelyServerTLS is the extension name enum for a server TLS config.
	XRelyServerTLS = "x-rely-server-tls"
	// XRelyProxyAction is the extension name enum for a proxy action.
	XRelyProxyAction = "x-rely-proxy-action"
	// XRelySecurityCredentials is the extension name enum for security credentials.
	XRelySecurityCredentials = "x-rely-security-credentials"
	// XRelyOAuth2TokenURLEnv is the extension name enum of a custom environment variable for OAuth2 token URL.
	XRelyOAuth2TokenURLEnv = "x-rely-oauth2-token-url-env" //nolint:gosec
	// XRelyOAuth2RefreshURLEnv is the extension name enum of a custom environment variable for OAuth2 refresh URL.
	XRelyOAuth2RefreshURLEnv = "x-rely-oauth2-refresh-url-env"
)

const (
	// Integer represents the constant of an integer type.
	Integer = "integer"
	// Number represents the constant of a number type.
	Number = "number"
	// Object represents the constant of an object type.
	Object = "object"
	// String represents the constant of a string type.
	String = "string"
	// Array represents the constant of an array type.
	Array = "array"
	// Boolean represents the constant of a boolean type.
	Boolean = "boolean"
)

const (
	// Pipe presents the constant of a pipe character.
	Pipe = "|"
	// Comma presents the constant of a comma character.
	Comma = ","
	// Space presents the constant of a space character.
	Space = " "
	// SemiColon presents the constant of a semicolon character.
	SemiColon = ";"
	// Asterisk presents the constant of a asterisk character.
	Asterisk = "*"
	// Dot presents the constant of a dot character.
	Dot = "."
	// Equals presents the constant of an equality character.
	Equals = "="
	// Slash presents the constant of aa slash character.
	Slash = "/"
)
