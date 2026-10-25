package cloud

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Public catalog images use GHCR. Custom registries require a pinned digest.
func pinImage(ctx context.Context, image, architecture string) (string, error) {
	if strings.Contains(image, "@sha256:") {
		return image, nil
	}
	if !strings.HasPrefix(image, "ghcr.io/") {
		return "", fmt.Errorf("pin custom image %s with @sha256:digest", image)
	}
	repository, tag, ok := strings.Cut(strings.TrimPrefix(image, "ghcr.io/"), ":")
	if !ok {
		tag = "latest"
	}
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "GET", "https://ghcr.io/token?service=ghcr.io&scope="+url.QueryEscape("repository:"+repository+":pull"), nil)
	if err != nil {
		return "", err
	}
	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	var token struct {
		Token string `json:"token"`
	}
	err = json.NewDecoder(io.LimitReader(res.Body, 65536)).Decode(&token)
	res.Body.Close()
	if res.StatusCode != 200 || err != nil || token.Token == "" {
		return "", fmt.Errorf("cannot authenticate public image %s", image)
	}
	fetch := func(reference string) ([]byte, string, error) {
		req, err := http.NewRequestWithContext(ctx, "GET", "https://ghcr.io/v2/"+repository+"/manifests/"+reference, nil)
		if err != nil {
			return nil, "", err
		}
		req.Header.Set("Authorization", "Bearer "+token.Token)
		req.Header.Set("Accept", "application/vnd.oci.image.index.v1+json, application/vnd.docker.distribution.manifest.list.v2+json, application/vnd.oci.image.manifest.v1+json, application/vnd.docker.distribution.manifest.v2+json")
		res, err := client.Do(req)
		if err != nil {
			return nil, "", err
		}
		defer res.Body.Close()
		if res.StatusCode != 200 {
			return nil, "", fmt.Errorf("image manifest %s returned HTTP %d", image, res.StatusCode)
		}
		data, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
		if err != nil {
			return nil, "", err
		}
		digest := sha256.Sum256(data)
		value := "sha256:" + hex.EncodeToString(digest[:])
		if expected := res.Header.Get("Docker-Content-Digest"); expected != "" && expected != value {
			return nil, "", fmt.Errorf("image manifest digest mismatch")
		}
		return data, value, nil
	}
	data, digest, err := fetch(tag)
	if err != nil {
		return "", err
	}
	var index struct {
		Manifests []struct {
			Digest   string
			Platform struct{ Architecture, OS string }
		}
	}
	if err = json.Unmarshal(data, &index); err != nil {
		return "", err
	}
	if len(index.Manifests) > 0 {
		digest = ""
		arch := architecture
		if arch == "x86_64" {
			arch = "amd64"
		}
		for _, m := range index.Manifests {
			if m.Platform.Architecture == arch && m.Platform.OS == "linux" {
				digest = m.Digest
				break
			}
		}
		if digest == "" {
			return "", fmt.Errorf("image %s has no Linux %s variant", image, arch)
		}
	}
	return "ghcr.io/" + repository + "@" + digest, nil
}
