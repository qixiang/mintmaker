package utils

import (
	"fmt"
	"net/url"
	"strings"
)

func GetGitPlatform(giturl string) (string, error) {
	allowedGitPlatforms := []string{"github", "gitlab"}
	host, err := GetGitHost(giturl)
	if err != nil {
		return "", err
	}

	var gitPlatform string
	for _, platform := range allowedGitPlatforms {
		if strings.Contains(host, platform) {
			gitPlatform = platform
			break
		}
	}
	if gitPlatform == "" {
		return "", fmt.Errorf("unsupported git platform for repository %s", giturl)
	}
	return gitPlatform, nil
}

func GetGitHost(giturl string) (string, error) {
	// Handle SSH URLs (user@host:path)
	if strings.Contains(giturl, "@") {
		parts := strings.SplitN(giturl, ":", 2)
		if len(parts) != 2 {
			return "", fmt.Errorf("invalid SSH URL format: %s", giturl)
		}
		hostPart := strings.SplitN(parts[0], "@", 2)
		if len(hostPart) != 2 {
			return "", fmt.Errorf("invalid SSH URL format: %s", giturl)
		}
		return hostPart[1], nil
	}

	u, err := url.Parse(giturl)
	if err != nil {
		return "", err
	}
	host := u.Hostname()

	return host, nil
}

func GetGitPath(giturl string) (string, error) {
	giturl = strings.TrimSuffix(strings.TrimSuffix(giturl, "/"), ".git")
	// Handle SSH URLs (user@host:path)
	if strings.Contains(giturl, "@") {
		parts := strings.SplitN(giturl, ":", 2)
		if len(parts) != 2 {
			return "", fmt.Errorf("invalid SSH URL format: %s", giturl)
		}
		return strings.TrimPrefix(parts[1], "/"), nil
	}

	u, err := url.Parse(giturl)
	if err != nil {
		return "", err
	}
	path := strings.TrimPrefix(u.Path, "/")
	return path, nil
}
