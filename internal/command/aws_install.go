package command

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fogo-sh/insta-game/internal/cloud"
	"golang.org/x/crypto/ssh"
)

type installOptions struct{ Binary, Engine, Assets, NativeConfig, Catalog string }
type installFile struct {
	Source, Destination string
	Mode                int64
}

func installHost(ctx context.Context, host *cloud.Host, options installOptions, out io.Writer) error {
	files := []installFile{
		{options.Binary, "usr/local/bin/insta-game.new", 0755},
		{options.Engine, "opt/insta-game/engine/" + filepath.Base(options.Engine), 0755},
		{options.NativeConfig, "opt/insta-game/native.json", 0600},
		{host.Config.TokenPath, "var/lib/insta-game/token", 0600},
	}
	for _, file := range files {
		info, err := os.Stat(file.Source)
		if err != nil {
			return fmt.Errorf("install file %q: %w", file.Source, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("install file %q is not regular", file.Source)
		}
	}
	for _, path := range []string{options.Assets, options.Catalog} {
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return fmt.Errorf("%s must be a directory", path)
		}
	}
	private, err := os.ReadFile(host.Config.KeyPath)
	if err != nil {
		return fmt.Errorf("read SSH private key: %w", err)
	}
	signer, err := ssh.ParsePrivateKey(private)
	if err != nil {
		return err
	}
	fmt.Fprintln(out, "Starting shared host and verifying its SSH identity...")
	ip, err := host.Wake(ctx)
	if err != nil {
		return err
	}
	hostKey, err := host.ConsoleHostKey(ctx)
	if err != nil {
		return err
	}
	client, err := dialHost(ctx, net.JoinHostPort(ip, "22"), signer, hostKey)
	if err != nil {
		return err
	}
	defer client.Close()
	cancelClose := context.AfterFunc(ctx, func() { client.Close() })
	defer cancelClose()
	if err := runSSH(client, `sudo cloud-init status --wait >/dev/null
sudo systemctl stop insta-game.service 2>/dev/null || true
sudo env DEBIAN_FRONTEND=noninteractive apt-get update -qq
sudo env DEBIAN_FRONTEND=noninteractive apt-get install -y -qq --no-install-recommends ca-certificates curl zlib1g libstdc++6
sudo install -d -m 0700 /var/lib/insta-game
sudo install -d -m 0755 /opt/insta-game/engine /opt/insta-game/assets /opt/insta-game/docker-containers`, nil, out); err != nil {
		return err
	}
	reader, writer := io.Pipe()
	sent := make(chan error, 1)
	go func() {
		archive := tar.NewWriter(writer)
		err := writeInstallFiles(archive, files)
		if err == nil {
			err = writeInstallTree(archive, options.Assets, "opt/insta-game/assets")
		}
		if err == nil {
			err = writeInstallTree(archive, options.Catalog, "opt/insta-game/docker-containers")
		}
		if closeErr := archive.Close(); err == nil {
			err = closeErr
		}
		writer.CloseWithError(err)
		sent <- err
	}()
	fmt.Fprintln(out, "Uploading binary, engine, assets, and catalog...")
	uploadErr := runSSH(client, "sudo tar -xf - -C /", reader, out)
	reader.CloseWithError(uploadErr)
	if err := <-sent; err != nil {
		return err
	}
	if uploadErr != nil {
		return uploadErr
	}
	if err := runSSH(client, installService, nil, out); err != nil {
		return err
	}
	fmt.Fprintf(out, "Shared controller installed at https://%s:3000 (token file: %s). Empty host stops after two minutes.\n", ip, host.Config.TokenPath)
	return nil
}

func dialHost(ctx context.Context, address string, signer ssh.Signer, key ssh.PublicKey) (*ssh.Client, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	config := &ssh.ClientConfig{User: "ubuntu", Auth: []ssh.AuthMethod{ssh.PublicKeys(signer)}, HostKeyCallback: ssh.FixedHostKey(key), HostKeyAlgorithms: []string{key.Type()}}
	var lastErr error
	for {
		conn, err := (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, "tcp", address)
		if err == nil {
			conn.SetDeadline(time.Now().Add(15 * time.Second))
			c, channels, requests, err := ssh.NewClientConn(conn, address, config)
			if err == nil {
				conn.SetDeadline(time.Time{})
				return ssh.NewClient(c, channels, requests), nil
			}
			conn.Close()
			return nil, fmt.Errorf("SSH authentication: %w", err)
		}
		lastErr = err
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("connect to SSH: %w (%v)", ctx.Err(), lastErr)
		case <-time.After(3 * time.Second):
		}
	}
}

func runSSH(client *ssh.Client, script string, input io.Reader, out io.Writer) error {
	session, err := client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()
	session.Stdin, session.Stdout, session.Stderr = input, out, out
	return session.Run("set -eu\n" + script)
}

func writeInstallFiles(archive *tar.Writer, files []installFile) error {
	for _, file := range files {
		if err := writeInstallFile(archive, file); err != nil {
			return err
		}
	}
	return nil
}

func writeInstallFile(archive *tar.Writer, file installFile) error {
	input, err := os.Open(file.Source)
	if err != nil {
		return err
	}
	defer input.Close()
	info, err := input.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("not a regular install file: %s", file.Source)
	}
	if err := archive.WriteHeader(&tar.Header{Name: file.Destination, Mode: file.Mode, Size: info.Size(), Typeflag: tar.TypeReg}); err != nil {
		return err
	}
	_, err = io.Copy(archive, input)
	return err
}

func writeInstallTree(archive *tar.Writer, source, destination string) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("install tree contains symlink: %s", path)
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(filepath.Join(destination, relative))
		if entry.IsDir() {
			return archive.WriteHeader(&tar.Header{Name: name + "/", Mode: 0755, Typeflag: tar.TypeDir})
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		return writeInstallFile(archive, installFile{path, name, int64(info.Mode().Perm() & 0755)})
	})
}

var installService = strings.TrimSpace(`
sudo sh <<'IDENTITY'
cat > /usr/local/sbin/insta-game-ssh-key <<'KEYSCRIPT'
#!/bin/sh
printf 'INSTA_GAME_SSH_HOST_KEY=%s\n' "$(cat /etc/ssh/ssh_host_ed25519_key.pub)" > /dev/console
KEYSCRIPT
chmod 0755 /usr/local/sbin/insta-game-ssh-key
cat > /etc/systemd/system/insta-game-ssh-key.service <<'KEYSERVICE'
[Unit]
Description=Publish the SSH host identity to the authenticated EC2 console
After=ssh.service
[Service]
Type=oneshot
ExecStart=/usr/local/sbin/insta-game-ssh-key
[Install]
WantedBy=multi-user.target
KEYSERVICE
systemctl daemon-reload
systemctl enable --now insta-game-ssh-key.service
IDENTITY
sudo mv -f /usr/local/bin/insta-game.new /usr/local/bin/insta-game
sudo chmod 0755 /usr/local/bin/insta-game
sudo tee /usr/local/bin/insta-game-host >/dev/null <<'SCRIPT'
#!/bin/sh
set -eu
metadata=http://169.254.169.254/latest
imds_token=$(curl --fail --silent --show-error --max-time 10 -X PUT -H 'X-aws-ec2-metadata-token-ttl-seconds: 60' "$metadata/api/token")
public_ip=$(curl --fail --silent --show-error --max-time 10 -H "X-aws-ec2-metadata-token: $imds_token" "$metadata/meta-data/public-ipv4")
exec /usr/local/bin/insta-game serve --listen 0.0.0.0:3000 --tls --poweroff-idle=2m --data /var/lib/insta-game --native-config /opt/insta-game/native.json --catalog /opt/insta-game/docker-containers --default-provider local --public-host "$public_ip" --game-bind 0.0.0.0
SCRIPT
sudo chmod 0755 /usr/local/bin/insta-game-host
sudo tee /etc/systemd/system/insta-game.service >/dev/null <<'SERVICE'
[Unit]
Description=Shared game host controller
Wants=network-online.target
After=network-online.target
[Service]
Type=simple
ExecStart=/usr/local/bin/insta-game-host
Restart=on-failure
RestartSec=5
KillMode=process
TimeoutStopSec=45
WorkingDirectory=/opt/insta-game
UMask=0077
[Install]
WantedBy=multi-user.target
SERVICE
sudo systemctl daemon-reload
sudo systemctl enable --now insta-game.service
sudo systemctl is-active insta-game.service
`)
