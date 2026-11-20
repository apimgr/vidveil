// SPDX-License-Identifier: MIT
package config

import (
	"bytes"
	"errors"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const serverConfigHeader = "# Settings of this node only (listen, identity, database, cache, ssl). Everything else lives in yaml.d/: branding.yml, web.yml, pages.yml, contact.yml, privacy.yml, security.yml, logs.yml, metrics.yml, notifications.yml, tor.yml, i2p.yml, maintenance.yml, accounts.yml, app.yml. Credentials live in secrets.yml.\n"

type configDocument struct {
	path     string
	readPath string
	name     string
	root     map[string]interface{}
}

func migrateConfigLayout(configPath string) (map[string]string, error) {
	baseInfo, statErr := os.Stat(configPath)
	if statErr != nil {
		return nil, fmt.Errorf("inspect server config for migration: %w", statErr)
	}
	if !baseInfo.Mode().IsRegular() {
		return nil, fmt.Errorf("%s must be a regular file", configPath)
	}
	if baseInfo.Mode().Perm() != 0600 {
		if err := os.Chmod(configPath, 0600); err != nil {
			return nil, fmt.Errorf("secure server config for migration: %w", err)
		}
	}
	base, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("read server config for migration: %w", err)
	}
	configDir := filepath.Dir(configPath)
	dropDir := filepath.Join(configDir, "yaml.d")
	if info, statErr := os.Lstat(dropDir); statErr == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return nil, fmt.Errorf("%s must be a real directory", dropDir)
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return nil, fmt.Errorf("inspect yaml.d for migration: %w", statErr)
	}
	documents := []configDocument{{path: configPath, readPath: configPath, name: "server.yml"}}
	var entries []os.DirEntry
	if _, err := os.Lstat(dropDir); err == nil {
		entries, err = os.ReadDir(dropDir)
		if err != nil {
			return nil, fmt.Errorf("read yaml.d for migration: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("inspect yaml.d for migration: %w", err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") || (!strings.HasSuffix(name, ".yml") && !strings.HasSuffix(name, ".yaml")) || strings.HasSuffix(name, ".bak") || strings.HasSuffix(name, "~") || strings.HasSuffix(name, ".orig") {
			continue
		}
		path := filepath.Join(dropDir, name)
		readPath := path
		if _, statErr := os.Lstat(path); statErr != nil {
			return nil, fmt.Errorf("inspect %s for migration: %w", path, statErr)
		} else if info, _ := os.Lstat(path); info.Mode()&os.ModeSymlink != 0 {
			readPath, err = safeConfigDropinTarget(path)
			if err != nil {
				return nil, fmt.Errorf("resolve %s for migration: %w", path, err)
			}
		}
		if _, readErr := os.ReadFile(readPath); readErr != nil {
			return nil, fmt.Errorf("read %s for migration: %w", path, readErr)
		}
		documents = append(documents, configDocument{path: path, readPath: readPath, name: name})
	}
	secretsPath := filepath.Join(configDir, "secrets.yml")
	if _, readErr := os.Lstat(secretsPath); readErr == nil {
		info, statErr := os.Lstat(secretsPath)
		if statErr != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return nil, fmt.Errorf("%s must be a regular file", secretsPath)
		}
		if info.Mode().Perm() != 0600 {
			if chmodErr := os.Chmod(secretsPath, 0600); chmodErr != nil {
				return nil, fmt.Errorf("secure %s: %w", secretsPath, chmodErr)
			}
		}
		documents = append(documents, configDocument{path: secretsPath, readPath: secretsPath, name: "secrets.yml"})
	} else if !errors.Is(readErr, os.ErrNotExist) {
		return nil, fmt.Errorf("read secrets.yml for migration: %w", readErr)
	}

	outputs := make(map[string]map[string]interface{})
	secretValues := make(map[string]interface{})
	changed := false
	pending := make(map[string]string)
	for index := range documents {
		doc := &documents[index]
		data, readErr := os.ReadFile(doc.readPath)
		if readErr != nil {
			return nil, fmt.Errorf("read %s for migration: %w", doc.path, readErr)
		}
		root, decodeErr := decodeConfigMap(data)
		if decodeErr != nil {
			return nil, fmt.Errorf("parse %s for migration: %w", doc.path, decodeErr)
		}
		doc.root = root
		if migrateLegacyKeys(root, pending) {
			changed = true
		}
		secrets := extractSecretSettings(root)
		if len(secrets) > 0 {
			mergeConfigMap(secretValues, secrets)
			if doc.name != "secrets.yml" {
				changed = true
			}
		}
		parts, moved := partitionConfig(root, doc.name)
		if moved {
			changed = true
		}
		for filename, part := range parts {
			if outputs[filename] == nil {
				outputs[filename] = make(map[string]interface{})
			}
			mergeConfigMap(outputs[filename], part)
		}
	}
	if len(pending) > 0 {
		changed = true
	}
	if !changed {
		return pending, nil
	}
	if err := os.MkdirAll(dropDir, 0700); err != nil {
		return nil, fmt.Errorf("create yaml.d for migration: %w", err)
	}
	if err := os.Chmod(dropDir, 0700); err != nil {
		return nil, fmt.Errorf("secure yaml.d for migration: %w", err)
	}

	backupPath := configPath + ".bak"
	if err := atomicConfigWrite(backupPath, base); err != nil {
		return nil, fmt.Errorf("back up server config before migration: %w", err)
	}
	if err := os.Chmod(backupPath, 0600); err != nil {
		return nil, fmt.Errorf("secure server config backup: %w", err)
	}
	for _, name := range orderedAreaFiles {
		root := outputs[name]
		path := filepath.Join(dropDir, name)
		if len(root) == 0 {
			if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
				return nil, fmt.Errorf("remove empty area file %s: %w", path, err)
			}
			continue
		}
		data, marshalErr := marshalYAML(root)
		if marshalErr != nil {
			return nil, fmt.Errorf("encode %s: %w", path, marshalErr)
		}
		if err := atomicConfigWrite(path, data); err != nil {
			return nil, fmt.Errorf("write %s: %w", path, err)
		}
	}
	if len(secretValues) > 0 {
		data, marshalErr := marshalYAML(secretValues)
		if marshalErr != nil {
			return nil, fmt.Errorf("encode secrets.yml: %w", marshalErr)
		}
		if err := atomicConfigWrite(secretsPath, data); err != nil {
			return nil, fmt.Errorf("write secrets.yml: %w", err)
		}
	}
	serverPart := outputs["server.yml"]
	serverData, err := marshalYAML(serverPart)
	if err != nil {
		return nil, fmt.Errorf("encode server.yml: %w", err)
	}
	if err := atomicConfigWrite(configPath, append([]byte(serverConfigHeader+"\n"), serverData...)); err != nil {
		return nil, fmt.Errorf("write migrated server.yml: %w", err)
	}
	for _, doc := range documents[1:] {
		if doc.name == "secrets.yml" || isCanonicalAreaPath(doc.name) {
			continue
		}
		if err := os.Remove(doc.path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("remove migrated drop-in %s: %w", doc.path, err)
		}
	}
	return pending, nil
}

var orderedAreaFiles = []string{
	"branding.yml", "web.yml", "pages.yml", "contact.yml", "privacy.yml", "security.yml", "logs.yml", "metrics.yml", "notifications.yml", "tor.yml", "i2p.yml", "maintenance.yml", "accounts.yml", "app.yml",
}

func decodeConfigMap(data []byte) (map[string]interface{}, error) {
	root := make(map[string]interface{})
	if err := yaml.NewDecoder(bytes.NewReader(data)).Decode(&root); err != nil {
		return nil, err
	}
	if root == nil {
		root = make(map[string]interface{})
	}
	return root, nil
}

func partitionConfig(root map[string]interface{}, source string) (map[string]map[string]interface{}, bool) {
	parts := map[string]map[string]interface{}{"server.yml": {}}
	moved := false
	canonicalSource := normalizeAreaName(source)
	if strings.HasSuffix(source, ".yaml") && source != "server.yaml" {
		moved = true
	}
	for key, value := range root {
		switch key {
		case "server":
			section, _ := value.(map[string]interface{})
			for name, child := range section {
				file := serverAreaFile(name)
				if file != canonicalSource {
					moved = true
				}
				if parts[file] == nil {
					parts[file] = make(map[string]interface{})
				}
				if parts[file]["server"] == nil {
					parts[file]["server"] = make(map[string]interface{})
				}
				serverPart := parts[file]["server"].(map[string]interface{})
				serverPart[name] = child
			}
		case "web":
			web, _ := value.(map[string]interface{})
			for name, child := range web {
				file := fileForWebSource(name)
				if file != canonicalSource {
					moved = true
				}
				if parts[file] == nil {
					parts[file] = make(map[string]interface{})
				}
				if parts[file]["web"] == nil {
					parts[file]["web"] = make(map[string]interface{})
				}
				parts[file]["web"].(map[string]interface{})[name] = child
			}
		default:
			file := "app.yml"
			if key == "data" {
				file = "security.yml"
			}
			if file != canonicalSource {
				moved = true
			}
			if parts[file] == nil {
				parts[file] = make(map[string]interface{})
			}
			if parts[file][key] == nil {
				parts[file][key] = value
			}
		}
	}
	return parts, moved
}

func serverAreaFile(name string) string {
	switch name {
	case "port", "address", "fqdn", "mode", "api_version", "baseurl", "user", "group", "pidfile", "daemonize", "debug", "database", "cache", "ssl", "trusted_proxies", "cluster":
		return "server.yml"
	case "cors", "csrf", "url_detection", "compression", "healthz", "i18n":
		return "web.yml"
	case "branding", "seo":
		return "branding.yml"
	case "pages":
		return "pages.yml"
	case "contact":
		return "contact.yml"
	case "privacy", "compliance", "tracking":
		return "privacy.yml"
	case "security", "rate_limit", "geoip", "limits", "tokens":
		return "security.yml"
	case "logs":
		return "logs.yml"
	case "metrics":
		return "metrics.yml"
	case "notifications":
		return "notifications.yml"
	case "tor":
		return "tor.yml"
	case "i2p":
		return "i2p.yml"
	case "maintenance", "scheduler", "update", "auto_update", "backup", "restore":
		return "maintenance.yml"
	default:
		return "app.yml"
	}
}

func fileForWebSource(name string) string {
	if name == "ui" {
		return "branding.yml"
	}
	return "web.yml"
}

func normalizeAreaName(name string) string {
	if name == "server.yml" || name == "secrets.yml" {
		return name
	}
	return strings.TrimSuffix(strings.TrimSuffix(name, ".yaml"), ".yml") + ".yml"
}

func isCanonicalAreaPath(name string) bool {
	if strings.HasSuffix(name, ".yaml") {
		return false
	}
	for _, candidate := range orderedAreaFiles {
		if name == candidate {
			return true
		}
	}
	return false
}

func mergeConfigMap(dst, src map[string]interface{}) {
	for key, value := range src {
		incoming, incomingMap := value.(map[string]interface{})
		current, currentMap := dst[key].(map[string]interface{})
		if incomingMap && currentMap {
			mergeConfigMap(current, incoming)
			continue
		}
		dst[key] = value
	}
}

func pathValue(root map[string]interface{}, path ...string) (interface{}, bool) {
	var current interface{} = root
	for _, key := range path {
		mapping, ok := current.(map[string]interface{})
		if !ok {
			return nil, false
		}
		current, ok = mapping[key]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func setPathValue(root map[string]interface{}, value interface{}, path ...string) {
	current := root
	for _, key := range path[:len(path)-1] {
		child, ok := current[key].(map[string]interface{})
		if !ok {
			child = make(map[string]interface{})
			current[key] = child
		}
		current = child
	}
	current[path[len(path)-1]] = value
}

func deletePathValue(root map[string]interface{}, path ...string) bool {
	if len(path) == 0 {
		return false
	}
	current := root
	for _, key := range path[:len(path)-1] {
		child, ok := current[key].(map[string]interface{})
		if !ok {
			return false
		}
		current = child
	}
	if _, ok := current[path[len(path)-1]]; !ok {
		return false
	}
	delete(current, path[len(path)-1])
	return true
}

func moveConfigPath(root map[string]interface{}, from, to []string, onlyEmpty bool) bool {
	value, exists := pathValue(root, from...)
	if !exists {
		return false
	}
	if target, targetExists := pathValue(root, to...); !targetExists || onlyEmpty && isEmptyConfigValue(target) {
		setPathValue(root, value, to...)
	}
	deletePathValue(root, from...)
	return true
}

func isEmptyConfigValue(value interface{}) bool {
	switch v := value.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(v) == ""
	default:
		return false
	}
}

func moveConfigTree(root map[string]interface{}, from, to []string) bool {
	value, exists := pathValue(root, from...)
	if !exists {
		return false
	}
	oldMap, oldIsMap := value.(map[string]interface{})
	newValue, hasNew := pathValue(root, to...)
	if !hasNew {
		setPathValue(root, value, to...)
	} else if newMap, newIsMap := newValue.(map[string]interface{}); oldIsMap && newIsMap {
		mergeMissingConfigMap(newMap, oldMap)
	}
	deletePathValue(root, from...)
	return true
}

func mergeMissingConfigMap(dst, src map[string]interface{}) {
	for key, value := range src {
		if existing, ok := dst[key]; ok {
			oldMap, oldIsMap := value.(map[string]interface{})
			newMap, newIsMap := existing.(map[string]interface{})
			if oldIsMap && newIsMap {
				mergeMissingConfigMap(newMap, oldMap)
			}
			continue
		}
		dst[key] = value
	}
}

func migrateLegacyKeys(root map[string]interface{}, pending map[string]string) bool {
	changed := false
	move := func(from, to []string, onlyEmpty bool) {
		if moveConfigPath(root, from, to, onlyEmpty) {
			changed = true
		}
	}
	moveTree := func(from, to []string) {
		if moveConfigTree(root, from, to) {
			changed = true
		}
	}
	moveTree([]string{"server", "logging"}, []string{"server", "logs"})
	move([]string{"server", "logs", "access_format"}, []string{"server", "logs", "access", "format"}, false)
	moveTree([]string{"server", "schedule"}, []string{"server", "scheduler"})
	moveTree([]string{"server", "web", "cookie_consent", "message"}, []string{"server", "privacy", "consent", "message"})
	moveTree([]string{"server", "web", "cookie_consent", "policy_url"}, []string{"server", "privacy", "consent", "policy", "url"})
	for _, path := range [][]string{{"server", "web", "cookie_consent", "enabled"}, {"server", "web", "cookie_consent"}} {
		if deletePathValue(root, path...) {
			changed = true
		}
	}
	moveTree([]string{"server", "web"}, []string{"web"})
	move([]string{"server", "security_headers", "hsts"}, []string{"web", "hsts", "enabled"}, false)
	move([]string{"server", "security_headers", "hsts_max_age"}, []string{"web", "hsts", "max_age_seconds"}, false)
	if deletePathValue(root, "server", "security_headers") {
		changed = true
	}
	moveTree([]string{"server", "security", "hsts"}, []string{"web", "hsts"})
	moveTree([]string{"server", "security", "nel"}, []string{"web", "headers", "nel"})
	moveTree([]string{"server", "security", "headers", "clear_site_data"}, []string{"web", "headers", "clear_site_data"})
	if deletePathValue(root, "server", "security", "headers") {
		changed = true
	}
	moveTree([]string{"server", "security", "csrf"}, []string{"server", "csrf"})
	moveTree([]string{"web", "csrf"}, []string{"server", "csrf"})
	move([]string{"server", "title"}, []string{"server", "branding", "title"}, false)
	move([]string{"server", "description"}, []string{"server", "branding", "description"}, false)
	move([]string{"server", "seo", "default_title"}, []string{"server", "branding", "title"}, true)
	move([]string{"server", "seo", "default_description"}, []string{"server", "branding", "description"}, true)
	move([]string{"server", "seo", "opengraph", "image"}, []string{"server", "seo", "og_image"}, true)
	move([]string{"server", "seo", "twitter", "site"}, []string{"server", "seo", "twitter_handle"}, true)
	for _, path := range [][]string{{"server", "seo", "opengraph", "description"}, {"server", "seo", "enabled"}, {"server", "seo", "sitemap", "enabled"}, {"server", "seo", "feed", "enabled"}} {
		if deletePathValue(root, path...) {
			changed = true
		}
	}
	move([]string{"server", "branding", "logo_url"}, []string{"server", "branding", "logo"}, false)
	move([]string{"server", "branding", "favicon_url"}, []string{"server", "branding", "favicon"}, false)
	move([]string{"server", "branding", "primary_color"}, []string{"web", "ui", "accent"}, false)
	if text, ok := pathValue(root, "server", "branding", "footer_text"); ok {
		if _, exists := pathValue(root, "server", "branding", "content", "footer"); !exists {
			setPathValue(root, map[string]interface{}{"format": "html", "body": "<p>" + htmlEscape(fmt.Sprint(text)) + "</p>"}, "server", "branding", "content", "footer")
		}
		deletePathValue(root, "server", "branding", "footer_text")
		changed = true
	}
	for _, path := range [][]string{{"server", "branding", "theme_color"}, {"server", "branding", "source_code_url"}} {
		if deletePathValue(root, path...) {
			changed = true
		}
	}
	move([]string{"web", "favicon_url"}, []string{"server", "branding", "favicon"}, false)
	move([]string{"web", "ui", "favicon"}, []string{"server", "branding", "favicon"}, false)
	move([]string{"web", "ui", "logo"}, []string{"server", "branding", "logo"}, false)
	if old, ok := pathValue(root, "web", "footer", "custom_html"); ok {
		if _, exists := pathValue(root, "server", "branding", "content", "footer"); !exists {
			setPathValue(root, map[string]interface{}{"format": "html", "body": old}, "server", "branding", "content", "footer")
		}
		deletePathValue(root, "web", "footer", "custom_html")
		changed = true
	}
	for _, page := range []string{"about", "privacy", "terms"} {
		from := []string{"server", "pages", page, "content"}
		if value, ok := pathValue(root, from...); ok {
			setPathValue(root, map[string]interface{}{"format": "md", "body": value}, "server", "branding", "content", page)
			deletePathValue(root, from...)
			changed = true
		}
	}
	move([]string{"server", "admin", "email"}, []string{"server", "contact", "admin", "email"}, false)
	move([]string{"server", "admin", "path"}, []string{"server", "admin_path"}, false)
	for _, path := range [][]string{{"server", "admin", "password"}, {"server", "admin", "token"}, {"server", "admin", "username"}, {"server", "admin", "two_factor"}} {
		if deletePathValue(root, path...) {
			changed = true
		}
	}
	if httpsPort, ok := pathValue(root, "server", "https_port"); ok {
		port, _ := pathValue(root, "server", "port")
		portText := fmt.Sprint(port)
		httpsText := fmt.Sprint(httpsPort)
		httpsNumber, httpsErr := strconv.Atoi(httpsText)
		if currentNumber, currentErr := strconv.Atoi(portText); currentErr == nil && httpsErr == nil && httpsNumber > 0 && currentNumber > 0 {
			setPathValue(root, fmt.Sprintf("%s,%s", portText, httpsText), "server", "port")
		}
		deletePathValue(root, "server", "https_port")
		changed = true
	}
	move([]string{"server", "ssl", "letsencrypt", "provider"}, []string{"server", "ssl", "letsencrypt", "dns_provider"}, false)
	move([]string{"server", "ssl", "letsencrypt", "credentials_encrypted"}, []string{"server", "ssl", "letsencrypt", "dns_credentials"}, false)
	moveTree([]string{"server", "ssl", "dns01"}, []string{"server", "ssl", "letsencrypt"})
	for _, path := range [][]string{{"server", "ssl", "letsencrypt", "validated_at"}, {"server", "ssl", "letsencrypt", "domain"}, {"server", "ssl", "letsencrypt", "domains"}} {
		if deletePathValue(root, path...) {
			changed = true
		}
	}
	for _, key := range []string{"installation_secret", "installation_secret_previous", "installation_secret_previous_expires", "encryption_key_previous", "encryption_key_previous_expires"} {
		path := []string{"server", "security", key}
		if value, ok := pathValue(root, path...); ok {
			pending[key] = fmt.Sprint(value)
			deletePathValue(root, path...)
			changed = true
		}
	}
	removed := [][]string{
		{"server", "privacy", "content"}, {"server", "mcp"}, {"server", "rate_limit", "enabled"}, {"server", "rate_limit", "requests"}, {"server", "rate_limit", "window"},
		{"server", "maintenance_mode"}, {"server", "i18n", "auto_detect"}, {"server", "i18n", "rtl_languages"}, {"server", "i18n", "show_selector"}, {"server", "i18n", "translations_dir"},
		{"server", "geoip", "dir"}, {"server", "geoip", "databases"}, {"server", "geoip", "country_mode"}, {"server", "database", "sqlite"}, {"server", "metrics", "endpoint"},
		{"server", "notifications", "email_enabled"}, {"server", "notifications", "webhook_enabled"}, {"server", "secret_key"}, {"web", "permissions_policy"}, {"web", "honor_dnt"}, {"web", "robots", "ai_bots"},
	}
	for _, path := range removed {
		if deletePathValue(root, path...) {
			changed = true
		}
	}
	if sanitizeSchedulerTasks(root) {
		changed = true
	}
	return changed
}

func sanitizeSchedulerTasks(root map[string]interface{}) bool {
	changed := false
	if scheduler, ok := pathValue(root, "server", "scheduler"); ok {
		if settings, ok := scheduler.(map[string]interface{}); ok {
			for key := range settings {
				if key != "timezone" && key != "catch_up_window" && key != "tasks" {
					delete(settings, key)
					changed = true
				}
			}
		}
	}
	tasks, ok := pathValue(root, "server", "scheduler", "tasks")
	if !ok {
		return changed
	}
	taskMap, ok := tasks.(map[string]interface{})
	if !ok {
		deletePathValue(root, "server", "scheduler", "tasks")
		return true
	}
	for name, value := range taskMap {
		settings, isMap := value.(map[string]interface{})
		if !isMap {
			delete(taskMap, name)
			changed = true
			continue
		}
		allowed := map[string]bool{"enabled": true, "schedule": true, "retry_on_fail": true, "retry_delay": true}
		if name == "backup" {
			allowed["retention"] = true
		}
		for key := range settings {
			if !allowed[key] {
				delete(settings, key)
				changed = true
			}
		}
		if defaultTask, exists := DefaultAppConfig().Server.Schedule.Tasks[name]; exists {
			defaultTask.Verify = false
			defaultTask.RestartOnFail = false
			if name != "backup" {
				defaultTask.Retention = nil
			}
			encoded, err := marshalYAML(settings)
			if err == nil {
				configured := defaultTask
				if yaml.Unmarshal(encoded, &configured) == nil && reflect.DeepEqual(configured, defaultTask) {
					delete(taskMap, name)
					changed = true
				}
			}
		}
	}
	return changed
}

func htmlEscape(value string) string {
	return html.EscapeString(value)
}

func extractSecretSettings(root map[string]interface{}) map[string]interface{} {
	secrets := make(map[string]interface{})
	for _, path := range [][]string{
		{"server", "token"}, {"server", "security", "encryption_key"}, {"server", "security", "encryption_key_version"},
		{"server", "notifications", "email", "smtp", "password"}, {"server", "cache", "password"},
		{"server", "database", "password"}, {"server", "database", "token"}, {"server", "pages", "contact", "captcha_secret_key"},
		{"server", "ssl", "letsencrypt", "dns_credentials"}, {"data", "cve", "api_key"}, {"server", "backup", "encryption_password"},
		{"server", "metrics", "auth", "tokens"}, {"server", "contact", "admin", "webhooks"}, {"server", "contact", "security", "webhooks"},
		{"server", "contact", "abuse", "webhooks"}, {"server", "contact", "general", "webhooks"},
	} {
		if value, ok := pathValue(root, path...); ok {
			setPathValue(secrets, value, path...)
			deletePathValue(root, path...)
		}
	}
	return secrets
}
