// Package cookies provides secure cookie helpers and policy-driven defaults.
//
// The package centers around `Manager`, which encapsulates signing/encryption
// key access and environment-aware defaults (SameSite, partitioning, HttpOnly).
// Callers choose explicit cookie config types for host-only vs non-host-only
// behavior and client-readable vs secure cookies.
//
// This package is intended to make secure defaults easy while keeping each
// cookie's behavior explicit at call sites.
package cookies

import (
	"fmt"
	"net/http"
	"time"

	"github.com/vormadev/vorma/kit/keyset"
	"github.com/vormadev/vorma/kit/response"
	"github.com/vormadev/vorma/kit/securestring"
)

type (
	// SameSite configures the cookie SameSite attribute.
	SameSite int
	// PartitionOption controls whether a cookie uses CHIPS partitioning.
	PartitionOption int
	// HttpOnlyOption controls whether secure cookies are HttpOnly.
	HttpOnlyOption int
)

const (
	sameSiteDefault SameSite = 0 // Use manager default
	// SameSiteLaxMode sets SameSite=Lax.
	SameSiteLaxMode SameSite = SameSite(http.SameSiteLaxMode)
	// SameSiteStrictMode sets SameSite=Strict.
	SameSiteStrictMode SameSite        = SameSite(http.SameSiteStrictMode)
	partitionDefault   PartitionOption = 0 // Use manager default
	// PartitionTrue explicitly enables partitioned cookies.
	PartitionTrue PartitionOption = 1 // Explicitly enable partitioning
	// PartitionFalse explicitly disables partitioned cookies.
	PartitionFalse  PartitionOption = 2 // Explicitly disable partitioning
	httpOnlyDefault HttpOnlyOption  = 0 // Use manager default
	// HttpOnlyTrue explicitly enables HttpOnly.
	HttpOnlyTrue HttpOnlyOption = 1 // Explicitly enable HttpOnly
	// HttpOnlyFalse explicitly disables HttpOnly.
	HttpOnlyFalse HttpOnlyOption = 2 // Explicitly disable HttpOnly
)

// Manager centralizes cookie defaults and runtime environment policy.
type Manager struct {
	cfg *ManagerConfig
}

// ManagerConfig configures defaults for cookies created by a Manager.
type ManagerConfig struct {
	GetKeyset func() *keyset.Keyset
	GetIsDev  func() bool // Optional. Resolves to false if nil.
	// The manager's default SameSite setting.
	DefaultSameSite SameSite
	// The manager's default for cookie partitioning.
	DefaultPartition PartitionOption
	// The manager's default for the HttpOnly flag on secure cookies.
	DefaultHttpOnly HttpOnlyOption
}

// NewManager validates cfg and returns a configured cookie manager.
func NewManager(cfg ManagerConfig) *Manager {
	if cfg.GetKeyset == nil {
		panic("GetKeyset function cannot be nil")
	}
	if cfg.DefaultSameSite == sameSiteDefault {
		cfg.DefaultSameSite = SameSiteLaxMode
	}
	if cfg.DefaultPartition == partitionDefault {
		cfg.DefaultPartition = PartitionTrue
	}
	if cfg.DefaultHttpOnly == httpOnlyDefault {
		cfg.DefaultHttpOnly = HttpOnlyTrue
	}
	return &Manager{cfg: &cfg}
}

// IsDev reports whether the manager is running in development mode.
func (mgr *Manager) IsDev() bool {
	return mgr.cfg.GetIsDev != nil && mgr.cfg.GetIsDev()
}

// SecureCookieConfig defines a host-prefixed encrypted cookie.
type SecureCookieConfig struct {
	Manager *Manager // Required.
	// Required. Do not prefix the name with "__Host-". Prefixing is handled internally.
	// Final cookie name will be "__{Host|Dev}-{Name}".
	Name      string
	TTL       time.Duration
	SameSite  SameSite
	Partition PartitionOption
	HttpOnly  HttpOnlyOption
}

// SecureCookieNonHostOnlyConfig defines an encrypted cookie without host prefix constraints.
type SecureCookieNonHostOnlyConfig struct {
	Manager   *Manager // Required.
	Name      string
	TTL       time.Duration
	SameSite  SameSite
	Partition PartitionOption
	Path      string
	Domain    string
	HttpOnly  HttpOnlyOption
}

// ClientReadableCookieConfig defines a host-prefixed plaintext cookie.
type ClientReadableCookieConfig struct {
	Manager *Manager // Required.
	// Required. Do not prefix the name with "__Host-". Prefixing is handled internally.
	// Final cookie name will be "__{Host|Dev}-{Name}".
	Name      string
	TTL       time.Duration
	SameSite  SameSite
	Partition PartitionOption
}

// ClientReadableCookieNonHostOnlyConfig defines a plaintext cookie without host prefix constraints.
type ClientReadableCookieNonHostOnlyConfig struct {
	Manager   *Manager // Required.
	Name      string
	TTL       time.Duration
	SameSite  SameSite
	Partition PartitionOption
	Path      string
	Domain    string
}

type cookieSpec struct {
	name          string
	value         string
	path          string
	domain        string
	ttl           time.Duration
	sameSite      http.SameSite
	httpOnly      bool
	useHostPrefix bool
	partitioned   bool
}

func (mgr *Manager) resolveSameSite(configured SameSite) http.SameSite {
	finalOption := configured
	if finalOption == sameSiteDefault {
		finalOption = mgr.cfg.DefaultSameSite
	}
	return http.SameSite(finalOption)
}

func (mgr *Manager) resolvePartition(configured PartitionOption) bool {
	finalOption := configured
	if finalOption == partitionDefault {
		finalOption = mgr.cfg.DefaultPartition
	}
	return finalOption == PartitionTrue
}

func (mgr *Manager) resolveHttpOnly(configured HttpOnlyOption) bool {
	finalOption := configured
	if finalOption == httpOnlyDefault {
		finalOption = mgr.cfg.DefaultHttpOnly
	}
	return finalOption == HttpOnlyTrue
}

func (mgr *Manager) buildCookie(spec cookieSpec) *http.Cookie {
	name := spec.name
	path := spec.path
	domain := spec.domain
	secure := !mgr.IsDev()
	partitioned := spec.partitioned && !mgr.IsDev()

	if spec.useHostPrefix {
		name = mgr.hostPrefixName(spec.name)
		if !mgr.IsDev() {
			secure = true
			domain = ""
			path = "/"
		}
	}

	return &http.Cookie{
		Name:        name,
		Value:       spec.value,
		Path:        path,
		Domain:      domain,
		Secure:      secure,
		SameSite:    spec.sameSite,
		HttpOnly:    spec.httpOnly,
		Partitioned: partitioned,
		MaxAge:      int(spec.ttl.Seconds()),
	}
}

func (mgr *Manager) hostPrefixName(name string) string {
	if mgr.IsDev() {
		return "__Dev-" + name
	}
	return "__Host-" + name
}

func resolvePath(path string) string {
	if path == "" {
		return "/"
	}
	return path
}

type secureCookie[T any] struct {
	mgr  *Manager
	spec cookieSpec
}

func (c *secureCookie[T]) New(data T) (*http.Cookie, error) {
	encrypted, err := securestring.Serialize(c.mgr.cfg.GetKeyset(), data)
	if err != nil {
		return nil, fmt.Errorf("failed to encode cookie value: %w", err)
	}
	spec := c.spec
	spec.value = string(encrypted)
	return c.mgr.buildCookie(spec), nil
}

func (c *secureCookie[T]) Get(r *http.Request) (T, error) {
	name := c.spec.name
	if c.spec.useHostPrefix {
		name = c.mgr.hostPrefixName(name)
	}

	cookie, err := r.Cookie(name)
	if err != nil {
		return *new(T), fmt.Errorf("failed to get cookie: %w", err)
	}
	if cookie.Value == "" {
		return *new(T), fmt.Errorf("cookie value is empty")
	}
	return securestring.Parse[T](
		c.mgr.cfg.GetKeyset(),
		securestring.SecureString(cookie.Value),
	)
}

func (c *secureCookie[T]) NewDeletion() *http.Cookie {
	spec := c.spec
	spec.value = ""
	cookie := c.mgr.buildCookie(spec)
	cookie.MaxAge = -1
	return cookie
}

func (c *secureCookie[T]) SetWithProxy(rp *response.Proxy, value T) error {
	cookie, err := c.New(value)
	if err != nil {
		return fmt.Errorf("failed to create secure cookie: %w", err)
	}
	rp.SetCookie(cookie)
	return nil
}
func (c *secureCookie[T]) SetWithWriter(w http.ResponseWriter, value T) error {
	cookie, err := c.New(value)
	if err != nil {
		return fmt.Errorf("failed to create secure cookie: %w", err)
	}
	http.SetCookie(w, cookie)
	return nil
}
func (c *secureCookie[T]) DeleteWithProxy(rp *response.Proxy) {
	cookie := c.NewDeletion()
	rp.SetCookie(cookie)
}
func (c *secureCookie[T]) DeleteWithWriter(w http.ResponseWriter) {
	cookie := c.NewDeletion()
	http.SetCookie(w, cookie)
}

func (c *secureCookie[T]) Name() string {
	if c.spec.useHostPrefix {
		return c.mgr.hostPrefixName(c.spec.name)
	}
	return c.spec.name
}

type clientReadableCookie[T ~string] struct {
	mgr  *Manager
	spec cookieSpec
}

func (c *clientReadableCookie[T]) New(value T) *http.Cookie {
	spec := c.spec
	spec.value = string(value)
	return c.mgr.buildCookie(spec)
}

func (c *clientReadableCookie[T]) Get(r *http.Request) (T, error) {
	name := c.spec.name
	if c.spec.useHostPrefix {
		name = c.mgr.hostPrefixName(name)
	}

	cookie, err := r.Cookie(name)
	if err != nil {
		return "", fmt.Errorf("failed to get cookie: %w", err)
	}
	return T(cookie.Value), nil
}

func (c *clientReadableCookie[T]) NewDeletion() *http.Cookie {
	spec := c.spec
	spec.value = ""
	cookie := c.mgr.buildCookie(spec)
	cookie.MaxAge = -1
	return cookie
}

func (c *clientReadableCookie[T]) SetWithProxy(rp *response.Proxy, value T) {
	cookie := c.New(value)
	rp.SetCookie(cookie)
}

func (c *clientReadableCookie[T]) SetWithWriter(
	w http.ResponseWriter,
	value T,
) {
	cookie := c.New(value)
	http.SetCookie(w, cookie)
}
func (c *clientReadableCookie[T]) DeleteWithProxy(rp *response.Proxy) {
	cookie := c.NewDeletion()
	rp.SetCookie(cookie)
}
func (c *clientReadableCookie[T]) DeleteWithWriter(w http.ResponseWriter) {
	cookie := c.NewDeletion()
	http.SetCookie(w, cookie)
}

func (c *clientReadableCookie[T]) Name() string {
	if c.spec.useHostPrefix {
		return c.mgr.hostPrefixName(c.spec.name)
	}
	return c.spec.name
}

// SecureCookie stores encrypted structured data in a host-prefixed cookie.
type SecureCookie[T any] struct {
	secureCookie[T]
}

// NewSecureCookie creates a host-prefixed secure cookie helper.
// It panics when Manager or Name is missing.
func NewSecureCookie[T any](cfg SecureCookieConfig) *SecureCookie[T] {
	if cfg.Manager == nil {
		panic("NewSecureCookie: Manager is required.")
	}
	if cfg.Name == "" {
		panic("NewSecureCookie: Name is required.")
	}
	spec := cookieSpec{
		name:          cfg.Name,
		path:          "/",
		domain:        "",
		ttl:           cfg.TTL,
		sameSite:      cfg.Manager.resolveSameSite(cfg.SameSite),
		httpOnly:      cfg.Manager.resolveHttpOnly(cfg.HttpOnly),
		useHostPrefix: true,
		partitioned:   cfg.Manager.resolvePartition(cfg.Partition),
	}
	return &SecureCookie[T]{secureCookie[T]{mgr: cfg.Manager, spec: spec}}
}

// SecureCookieNonHostOnly stores encrypted structured data without host-prefix constraints.
type SecureCookieNonHostOnly[T any] struct {
	secureCookie[T]
}

// NewSecureCookieNonHostOnly creates a non-host-only secure cookie helper.
// It panics when Manager or Name is missing.
func NewSecureCookieNonHostOnly[T any](
	cfg SecureCookieNonHostOnlyConfig,
) *SecureCookieNonHostOnly[T] {
	if cfg.Manager == nil {
		panic("NewSecureCookieNonHostOnly: Manager is required.")
	}
	if cfg.Name == "" {
		panic("NewSecureCookieNonHostOnly: Name is required.")
	}
	spec := cookieSpec{
		name:          cfg.Name,
		path:          resolvePath(cfg.Path),
		domain:        cfg.Domain,
		ttl:           cfg.TTL,
		sameSite:      cfg.Manager.resolveSameSite(cfg.SameSite),
		httpOnly:      cfg.Manager.resolveHttpOnly(cfg.HttpOnly),
		useHostPrefix: false,
		partitioned:   cfg.Manager.resolvePartition(cfg.Partition),
	}
	return &SecureCookieNonHostOnly[T]{
		secureCookie[T]{mgr: cfg.Manager, spec: spec},
	}
}

// ClientReadableCookie stores plaintext string data in a host-prefixed cookie.
type ClientReadableCookie[T ~string] struct {
	clientReadableCookie[T]
}

// NewClientReadableCookie creates a host-prefixed client-readable cookie helper.
// It panics when Manager or Name is missing.
func NewClientReadableCookie[T ~string](
	cfg ClientReadableCookieConfig,
) *ClientReadableCookie[T] {
	if cfg.Manager == nil {
		panic("NewClientReadableCookie: Manager is required.")
	}
	if cfg.Name == "" {
		panic("NewClientReadableCookie: Name is required.")
	}
	spec := cookieSpec{
		name:          cfg.Name,
		path:          "/",
		domain:        "",
		ttl:           cfg.TTL,
		sameSite:      cfg.Manager.resolveSameSite(cfg.SameSite),
		httpOnly:      false, // Always false for client-readable cookies
		useHostPrefix: true,
		partitioned:   cfg.Manager.resolvePartition(cfg.Partition),
	}
	return &ClientReadableCookie[T]{
		clientReadableCookie[T]{mgr: cfg.Manager, spec: spec},
	}
}

// ClientReadableCookieNonHostOnly stores plaintext string data without host-prefix constraints.
type ClientReadableCookieNonHostOnly[T ~string] struct {
	clientReadableCookie[T]
}

// NewClientReadableCookieNonHostOnly creates a non-host-only client-readable cookie helper.
// It panics when Manager or Name is missing.
func NewClientReadableCookieNonHostOnly[T ~string](
	cfg ClientReadableCookieNonHostOnlyConfig,
) *ClientReadableCookieNonHostOnly[T] {
	if cfg.Manager == nil {
		panic("NewClientReadableCookieNonHostOnly: Manager is required.")
	}
	if cfg.Name == "" {
		panic("NewClientReadableCookieNonHostOnly: Name is required.")
	}
	spec := cookieSpec{
		name:          cfg.Name,
		path:          resolvePath(cfg.Path),
		domain:        cfg.Domain,
		ttl:           cfg.TTL,
		sameSite:      cfg.Manager.resolveSameSite(cfg.SameSite),
		httpOnly:      false, // Always false for client-readable cookies
		useHostPrefix: false,
		partitioned:   cfg.Manager.resolvePartition(cfg.Partition),
	}
	return &ClientReadableCookieNonHostOnly[T]{
		clientReadableCookie[T]{mgr: cfg.Manager, spec: spec},
	}
}
