// SPDX-License-Identifier: MIT
// AI.md PART 31: Username and Email Validation
package validation

import (
	"fmt"
	"regexp"
	"strings"
)

// UsernameBlocklist per AI.md PART 31
// Server admin account is exempt from this blocklist
var UsernameBlocklist = []string{
	// System & Administrative
	"admin", "administrator", "root", "system", "sysadmin", "superuser",
	"master", "owner", "operator", "manager", "moderator", "mod",
	"staff", "support", "helpdesk", "help", "service", "daemon",

	// Server & Technical
	"server", "host", "node", "cluster", "api", "www", "web", "mail",
	"email", "smtp", "ftp", "ssh", "dns", "proxy", "gateway", "router",
	"firewall", "localhost", "local", "internal", "external", "public",
	"private", "network", "database", "db", "cache", "redis", "mysql",
	"postgres", "mongodb", "elastic", "nginx", "apache", "docker",

	// Application & Service Names
	"app", "application", "bot", "robot", "crawler", "spider", "scraper",
	"webhook", "callback", "cron", "scheduler", "worker", "queue", "job",
	"task", "process", "microservice", "lambda", "function",

	// Authentication & Security
	"auth", "authentication", "login", "logout", "signin", "signout",
	"signup", "register", "password", "passwd", "token", "oauth", "sso",
	"saml", "ldap", "kerberos", "security", "secure", "ssl", "tls",
	"certificate", "cert", "key", "secret", "credential", "session",

	// Roles & Permissions
	"guest", "anonymous", "anon", "user", "users", "member", "members",
	"subscriber", "editor", "author", "contributor", "reviewer", "auditor",
	"analyst", "developer", "dev", "devops", "engineer", "architect",
	"designer", "tester", "qa", "billing", "finance", "legal", "hr",
	"sales", "marketing", "ceo", "cto", "cfo", "coo", "founder", "cofounder",

	// Common Reserved
	"account", "accounts", "profile", "profiles", "settings", "config",
	"configuration", "dashboard", "panel", "console", "portal", "home",
	"index", "main", "default", "null", "nil", "undefined", "void",
	"true", "false", "test", "testing", "debug", "demo", "example",
	"sample", "temp", "temporary", "tmp", "backup", "archive", "log",
	"logs", "audit", "report", "reports", "analytics", "stats", "status",

	// API & Endpoints
	"rest", "graphql", "grpc", "websocket", "ws", "wss", "http",
	"https", "endpoint", "endpoints", "route", "routes", "path", "url",
	"uri", "hook", "hooks", "event", "events", "stream",

	// Content & Media
	"blog", "news", "article", "articles", "post", "posts", "page", "pages",
	"feed", "rss", "atom", "sitemap", "robots", "favicon", "static",
	"assets", "images", "image", "img", "media", "upload", "uploads",
	"download", "downloads", "file", "files", "document", "documents",

	// Communication
	"contact", "message", "messages", "chat", "notification", "notifications",
	"alert", "alerts", "inbox", "outbox", "sent", "draft", "drafts",
	"spam", "abuse", "flag", "block", "mute", "ban",

	// Commerce & Billing
	"shop", "store", "cart", "checkout", "order", "orders", "invoice",
	"invoices", "payment", "payments", "subscription", "subscriptions",
	"plan", "plans", "pricing", "refund", "coupon", "discount",

	// Social Features
	"follow", "follower", "followers", "following", "friend", "friends",
	"like", "likes", "share", "shares", "comment", "comments", "reply",
	"mention", "mentions", "tag", "tags", "group", "groups", "team", "teams",
	"community", "communities", "forum", "forums", "channel", "channels",

	// Brand & Legal
	"official", "verified", "trusted", "partner", "affiliate", "sponsor",
	"brand", "trademark", "copyright", "terms", "privacy",
	"policy", "policies", "tos", "eula", "gdpr", "dmca",

	// Common Spam Patterns
	"info", "noreply", "no-reply", "donotreply", "mailer", "postmaster",
	"webmaster", "hostmaster", "junk", "trash",

	// Project-specific
	"vidveil",
}

// CriticalTerms are checked as substrings (more strict)
var criticalTerms = []string{
	"admin", "root", "system", "mod", "official", "verified",
}

// Username validation regex per PART 31
// Allowed: a-z, 0-9, _, - (lowercase only)
// Must start with letter
// Cannot end with _ or -
// No consecutive special chars
var usernameRegex = regexp.MustCompile(`^[a-z][a-z0-9]*(?:[_-][a-z0-9]+)*$`)

// UsernameError represents a validation error
type UsernameError struct {
	Field   string
	Message string
}

func (e *UsernameError) Error() string {
	return e.Message
}

// ValidateUsername validates a username per AI.md PART 31
// Set isAdmin=true to exempt from blocklist (for server admin accounts)
func ValidateUsername(username string, isAdmin bool) error {
	// Convert to lowercase for validation
	username = strings.ToLower(username)

	// Check length
	if len(username) < 3 {
		return &UsernameError{
			Field:   "username",
			Message: "Username must be at least 3 characters",
		}
	}

	if len(username) > 32 {
		return &UsernameError{
			Field:   "username",
			Message: "Username cannot exceed 32 characters",
		}
	}

	// Check format
	if !usernameRegex.MatchString(username) {
		// Determine specific error
		if username[0] < 'a' || username[0] > 'z' {
			return &UsernameError{
				Field:   "username",
				Message: "Username must start with a letter",
			}
		}
		lastChar := username[len(username)-1]
		if lastChar == '_' || lastChar == '-' {
			return &UsernameError{
				Field:   "username",
				Message: "Username cannot end with underscore or hyphen",
			}
		}
		if strings.Contains(username, "__") || strings.Contains(username, "--") ||
			strings.Contains(username, "_-") || strings.Contains(username, "-_") {
			return &UsernameError{
				Field:   "username",
				Message: "Username cannot contain consecutive special characters",
			}
		}
		return &UsernameError{
			Field:   "username",
			Message: "Username can only contain lowercase letters, numbers, underscore, and hyphen",
		}
	}

	// Skip blocklist check for admin accounts
	if isAdmin {
		return nil
	}

	// Check blocklist (exact match)
	for _, blocked := range UsernameBlocklist {
		if username == blocked {
			return &UsernameError{
				Field:   "username",
				Message: "Username contains blocked word: " + blocked,
			}
		}
	}

	// Check critical terms as substrings
	for _, term := range criticalTerms {
		if strings.Contains(username, term) {
			return &UsernameError{
				Field:   "username",
				Message: "Username contains blocked word: " + term,
			}
		}
	}

	return nil
}

// ValidateEmail validates an email address
func ValidateEmail(email string) error {
	email = strings.ToLower(strings.TrimSpace(email))

	if email == "" {
		return &UsernameError{
			Field:   "email",
			Message: "Email address is required",
		}
	}

	// Basic email regex
	emailRegex := regexp.MustCompile(`^[a-z0-9._%+-]+@[a-z0-9.-]+\.[a-z]{2,}$`)
	if !emailRegex.MatchString(email) {
		return &UsernameError{
			Field:   "email",
			Message: "Please enter a valid email address",
		}
	}

	return nil
}

// ValidatePassword validates password strength per AI.md PART 22
// Minimum requirements: 8 chars, 1 upper, 1 lower, 1 number, 1 special
func ValidatePassword(password string) error {
	return ValidatePasswordWithPolicy(password, false)
}

// ValidateAdminPassword validates admin password with stricter requirements
// Minimum 12 characters for admin accounts
func ValidateAdminPassword(password string) error {
	return ValidatePasswordWithPolicy(password, true)
}

// ValidatePasswordWithPolicy validates password with configurable admin flag
func ValidatePasswordWithPolicy(password string, isAdmin bool) error {
	minLen := 8
	if isAdmin {
		minLen = 12
	}

	if len(password) < minLen {
		return &UsernameError{
			Field:   "password",
			Message: fmt.Sprintf("Password must be at least %d characters", minLen),
		}
	}

	var (
		hasUpper   bool
		hasLower   bool
		hasNumber  bool
		hasSpecial bool
	)

	for _, c := range password {
		switch {
		case 'A' <= c && c <= 'Z':
			hasUpper = true
		case 'a' <= c && c <= 'z':
			hasLower = true
		case '0' <= c && c <= '9':
			hasNumber = true
		case strings.ContainsRune("!@#$%^&*()_+-=[]{}|;':\",./<>?`~", c):
			hasSpecial = true
		}
	}

	if !hasUpper {
		return &UsernameError{
			Field:   "password",
			Message: "Password must contain at least one uppercase letter",
		}
	}
	if !hasLower {
		return &UsernameError{
			Field:   "password",
			Message: "Password must contain at least one lowercase letter",
		}
	}
	if !hasNumber {
		return &UsernameError{
			Field:   "password",
			Message: "Password must contain at least one number",
		}
	}
	if !hasSpecial {
		return &UsernameError{
			Field:   "password",
			Message: "Password must contain at least one special character",
		}
	}

	return nil
}
