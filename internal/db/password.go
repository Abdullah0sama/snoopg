package db

import (
	"fmt"
	"net/url"
	"os/exec"
	"strings"
)

func ResolvePassword(dsn, tokenCmd string) (string, error) {
	if tokenCmd == "" {
		return dsn, nil
	}
	out, err := exec.Command("sh", "-c", tokenCmd).Output()
	if err != nil {
		return "", fmt.Errorf("token command failed: %v", err)
	}
	token := strings.TrimSpace(string(out))
	if token == "" {
		return "", fmt.Errorf("token command returned empty output")
	}
	u, err := url.Parse(dsn)
	if err != nil {
		return "", fmt.Errorf("token_cmd requires a URL-form DSN: %v", err)
	}
	user := ""
	if u.User != nil {
		user = u.User.Username()
	}
	u.User = url.UserPassword(user, token)
	return u.String(), nil
}
