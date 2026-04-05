package main

import (
	"fmt"
	"os"
	"strings"
)

func configureRcon(protocol, configPath, password string) error {
	if password == "" {
		return nil
	}
	if strings.ContainsAny(password, "\r\n") {
		return fmt.Errorf("password contains newline")
	}

	switch protocol {
	case "xonotic", "quake1":
		return setConfigLine(configPath, "rcon_password", fmt.Sprintf("rcon_password %q", password))
	case "quake3":
		return setConfigLine(configPath, "seta rconPassword", fmt.Sprintf("seta rconPassword %q", password))
	case "quake2":
		return setConfigLine(configPath, "set rcon_password", fmt.Sprintf("set rcon_password %q", password))
	case "bzflag":
		return setConfigLine(configPath, "-passwd", fmt.Sprintf("-passwd %s", password))
	case "ut99":
		return setINIValue(configPath, "Engine.AccessControl", "AdminPassword", password)
	default:
		return nil
	}
}

func setConfigLine(path, directivePrefix, replacement string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	lines := strings.Split(string(content), "\n")
	found := false
	for i, line := range lines {
		if hasDirectivePrefix(line, directivePrefix) {
			if !found {
				lines[i] = replacement
				found = true
				continue
			}
			lines[i] = ""
		}
	}
	if !found {
		lines = appendConfigLine(lines, replacement)
	}

	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0644)
}

func hasDirectivePrefix(line, directivePrefix string) bool {
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "#") || trimmed == "" {
		return false
	}
	trimmed = strings.ToLower(trimmed)
	directivePrefix = strings.ToLower(directivePrefix)
	return trimmed == directivePrefix ||
		strings.HasPrefix(trimmed, directivePrefix+" ") ||
		strings.HasPrefix(trimmed, directivePrefix+"\t") ||
		strings.HasPrefix(trimmed, directivePrefix+"=")
}

func setINIValue(path, section, key, value string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	lines := strings.Split(string(content), "\n")
	sectionHeader := "[" + section + "]"
	inSection := false
	sectionFound := false
	keyFound := false
	insertIndex := len(lines)

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			if inSection && !keyFound {
				insertIndex = i
			}
			inSection = strings.EqualFold(trimmed, sectionHeader)
			sectionFound = sectionFound || inSection
			continue
		}
		if inSection && hasINIKey(line, key) {
			if !keyFound {
				lines[i] = fmt.Sprintf("%s=%s", key, value)
				keyFound = true
				continue
			}
			lines[i] = ""
		}
	}

	if sectionFound && !keyFound {
		lines = append(lines[:insertIndex], append([]string{fmt.Sprintf("%s=%s", key, value)}, lines[insertIndex:]...)...)
	}
	if !sectionFound {
		lines = appendConfigLine(lines, "")
		lines = append(lines, sectionHeader, fmt.Sprintf("%s=%s", key, value))
	}

	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0644)
}

func hasINIKey(line, key string) bool {
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, ";") || strings.HasPrefix(trimmed, "#") || trimmed == "" {
		return false
	}
	name, _, ok := strings.Cut(trimmed, "=")
	return ok && strings.EqualFold(strings.TrimSpace(name), key)
}

func appendConfigLine(lines []string, line string) []string {
	if len(lines) > 0 && lines[len(lines)-1] != "" {
		lines = append(lines, "")
	}
	return append(lines, line)
}
