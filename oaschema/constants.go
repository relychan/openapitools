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

// Type represents an enum of JSON schema type.
type Type string

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
	// Null represents the constant of a nullable type.
	Null = "null"
)

const (
	// Date represents the full-date notation as defined by RFC 3339, section 5.6, for example, 2017-07-21.
	Date = "date"
	// DateTime represents the date-time notation as defined by RFC 3339, section 5.6, for example, 2017-07-21T17:32:28Z.
	DateTime = "date-time"
	// Password represents a hint to UIs to mask the input.
	Password = "password"
	// Byte represents base64-encoded characters, for example, U3dhZ2dlciByb2Nrcw==.
	Byte = "byte"
	// Binary represents the binary data, used to describe files.
	Binary = "binary"
	// Email represents an email string. It is a hint to UIs to render the form.
	Email = "email"
	// UUID represents a UUID string.
	UUID = "uuid"
	// URI represents a URI string.
	URI = "uri"
	// Hostname represents an hostname string.
	Hostname = "hostname"
	// IPv4 represents an IPv4 string.
	IPv4 = "ipv4"
	// IPv6 represents an IPv6 string.
	IPv6 = "ipv6"
	// Float represents floating-point numbers.
	Float = "float"
	// Double represents floating-point numbers with double precision.
	Double = "double"
	// Int32 represents signed 32-bit integers (commonly used integer type).
	Int32 = "int32"
	// Int64 represents signed 64-bit integers (long type).
	Int64 = "int64"
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
