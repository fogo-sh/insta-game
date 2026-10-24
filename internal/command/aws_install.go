package command

import (
	"archive/tar"
	"context"
	"debug/elf"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/fogo-sh/insta-game/internal/cloud"
	"github.com/fogo-sh/insta-game/internal/control"
	"github.com/fogo-sh/insta-game/internal/lobby"
	"golang.org/x/crypto/ssh"
)

type installOptions struct {
	Binary, Engines, Assets, NativeConfig, Catalog, EnvFile string
	PullImages                                              bool
	MaxHostRuntime                                          time.Duration
}
type installFile struct {
	Source, Destination string
	Mode                int64
}

type preparedInstall struct {
	files        []installFile
	architecture string
	games        []control.Game
}

func prepareInstall(options installOptions, tokenPath string) (preparedInstall, error) {
	if options.MaxHostRuntime < 0 || (options.MaxHostRuntime > 0 && options.MaxHostRuntime < time.Minute) {
		return preparedInstall{}, fmt.Errorf("host-max-runtime must be zero or at least one minute")
	}
	games, err := control.LoadCatalog(options.Catalog)
	if err != nil {
		return preparedInstall{}, err
	}
	executable, err := elf.Open(options.Binary)
	if err != nil {
		return preparedInstall{}, fmt.Errorf("Linux ELF binary required: %w", err)
	}
	machine := executable.Machine
	executable.Close()
	architecture := ""
	switch machine {
	case elf.EM_X86_64:
		architecture = "x86_64"
	case elf.EM_AARCH64:
		architecture = "arm64"
	default:
		return preparedInstall{}, fmt.Errorf("unsupported controller architecture %s", machine)
	}
	if options.NativeConfig != "" {
		data, err := os.ReadFile(options.NativeConfig)
		if err != nil {
			return preparedInstall{}, err
		}
		var engines map[string]control.NativeGame
		if err := json.Unmarshal(data, &engines); err != nil {
			return preparedInstall{}, fmt.Errorf("native config: %w", err)
		}
		if len(engines) == 0 {
			return preparedInstall{}, fmt.Errorf("native config contains no engines")
		}
		known := map[string]bool{}
		for _, game := range games {
			known[game.ID] = true
		}
		for id, engine := range engines {
			if !known[id] || !filepath.IsAbs(engine.Command) || !filepath.IsAbs(engine.Assets) || !filepath.IsLocal(engine.ConfigPath) {
				return preparedInstall{}, fmt.Errorf("invalid native profile %q: require a catalog game, absolute command/assets, and relative configPath", id)
			}
		}
	}
	files := []installFile{
		{options.Binary, "usr/local/bin/insta-game.new", 0755},
		{tokenPath, "var/lib/insta-game/token", 0600},
	}
	if options.EnvFile != "" {
		files = append(files, installFile{options.EnvFile, "etc/insta-game.env", 0600})
	}
	metadata, _ := filepath.Glob(filepath.Join(options.Catalog, "*", "game.json"))
	for _, path := range metadata {
		directory := filepath.Dir(path)
		destination := "opt/insta-game/docker-containers/" + filepath.Base(directory) + "/"
		files = append(files, installFile{path, destination + "game.json", 0644})
		for _, name := range []string{"server.cfg", "UnrealTournament.ini"} {
			source := filepath.Join(directory, name)
			if _, err := os.Lstat(source); err == nil {
				files = append(files, installFile{source, destination + name, 0644})
			} else if !os.IsNotExist(err) {
				return preparedInstall{}, err
			}
		}
	}
	if options.NativeConfig != "" {
		files = append(files, installFile{options.NativeConfig, "opt/insta-game/native.json", 0600})
	}
	for _, file := range files {
		info, err := os.Lstat(file.Source)
		if err != nil {
			return preparedInstall{}, fmt.Errorf("install file %q: %w", file.Source, err)
		}
		if !info.Mode().IsRegular() {
			return preparedInstall{}, fmt.Errorf("install file %q is not regular", file.Source)
		}
	}
	for _, path := range []string{options.Engines, options.Assets} {
		if path == "" {
			continue
		}
		info, err := os.Stat(path)
		if err != nil {
			return preparedInstall{}, err
		}
		if !info.IsDir() {
			return preparedInstall{}, fmt.Errorf("%s must be a directory", path)
		}
		if err := filepath.WalkDir(path, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !entry.IsDir() && !entry.Type().IsRegular() {
				return fmt.Errorf("install tree contains nonregular file: %s", path)
			}
			return nil
		}); err != nil {
			return preparedInstall{}, err
		}
	}
	return preparedInstall{files: files, architecture: architecture, games: games}, nil
}

func installHost(ctx context.Context, host *cloud.Host, options installOptions, out io.Writer) error {
	prepared, err := prepareInstall(options, host.Config.TokenPath)
	if err != nil {
		return err
	}
	private, err := os.ReadFile(host.Config.KeyPath)
	if err != nil {
		return fmt.Errorf("read SSH private key: %w", err)
	}
	signer, err := ssh.ParsePrivateKey(private)
	if err != nil {
		return err
	}
	state, err := host.Inspect(ctx)
	if err != nil {
		return err
	}
	if state.Architecture != prepared.architecture {
		return fmt.Errorf("controller binary is %s; host is %s", prepared.architecture, state.Architecture)
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
	if err := runSSH(client, `cloud_init_status=0
sudo cloud-init status --wait || cloud_init_status=$?
case "$cloud_init_status" in
  0) ;;
  2) echo "cloud-init finished with recoverable warnings" ;;
  *) exit "$cloud_init_status" ;;
esac
if systemctl cat insta-game.service >/dev/null 2>&1; then sudo systemctl stop insta-game.service; fi
sudo env DEBIAN_FRONTEND=noninteractive apt-get update -qq
sudo env DEBIAN_FRONTEND=noninteractive apt-get install -y -qq --no-install-recommends ca-certificates curl zlib1g libstdc++6 docker.io
sudo systemctl enable --now docker
sudo install -d -m 0700 /var/lib/insta-game
sudo install -d -m 0755 /opt/insta-game/engine /opt/insta-game/assets /opt/insta-game/docker-containers`, nil, out); err != nil {
		return err
	}
	if options.PullImages {
		for _, game := range prepared.games {
			image := "'" + strings.ReplaceAll(game.Image, "'", "'\"'\"'") + "'"
			if err := runSSH(client, "sudo docker pull -- "+image, nil, out); err != nil {
				return fmt.Errorf("pull %s: %w", game.Image, err)
			}
		}
	}
	reader, writer := io.Pipe()
	sent := make(chan error, 1)
	go func() {
		archive := tar.NewWriter(writer)
		err := writeInstallFiles(archive, prepared.files)
		if err == nil && options.Engines != "" {
			err = writeInstallTree(archive, options.Engines, "opt/insta-game/engine")
		}
		if err == nil && options.Assets != "" {
			err = writeInstallTree(archive, options.Assets, "opt/insta-game/assets")
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
	lifetime := "sudo systemctl disable --now insta-game-deadline.timer"
	if options.MaxHostRuntime > 0 {
		lifetime = fmt.Sprintf(`sudo install -d /etc/systemd/system/insta-game-deadline.timer.d
sudo tee /etc/systemd/system/insta-game-deadline.timer.d/lifetime.conf >/dev/null <<'LIFETIME'
[Unit]
Description=Game host maximum boot lifetime
[Timer]
OnBootSec=
OnBootSec=%dms
LIFETIME
sudo systemctl daemon-reload
sudo systemctl enable insta-game-deadline.timer
sudo systemctl restart insta-game-deadline.timer`, options.MaxHostRuntime.Milliseconds())
	}
	if err := runSSH(client, lifetime, nil, out); err != nil {
		return err
	}
	lobbyEnv := "sudo rm -f /etc/insta-game-lobby.env"
	if host.Config.LobbyTable != "" {
		if _, err := lobby.ParseIDs(host.Config.LobbyInstances); err != nil {
			return err
		}
		if !regexp.MustCompile(`^[A-Za-z0-9_.-]+$`).MatchString(host.Config.LobbyTable) || !regexp.MustCompile(`^[a-z0-9-]+$`).MatchString(host.Config.Region) {
			return fmt.Errorf("invalid lobby table or region")
		}
		lobbyEnv = fmt.Sprintf("sudo tee /etc/insta-game-lobby.env >/dev/null <<'LOBBY'\nAWS_REGION=%s\nINSTA_GAME_LOBBY_TABLE=%s\nINSTA_GAME_LOBBY_INSTANCES=%s\nLOBBY\nsudo chmod 0600 /etc/insta-game-lobby.env", host.Config.Region, host.Config.LobbyTable, host.Config.LobbyInstances)
	}
	if err := runSSH(client, lobbyEnv, nil, out); err != nil {
		return err
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
set --
if [ -f /opt/insta-game/native.json ]; then
    set -- --native-config /opt/insta-game/native.json
fi
exec /usr/local/bin/insta-game serve "$@" --listen 0.0.0.0:3000 --tls --poweroff-idle=2m --data /var/lib/insta-game --catalog /opt/insta-game/docker-containers --default-provider docker --public-host "$public_ip" --game-bind 0.0.0.0
SCRIPT
sudo chmod 0755 /usr/local/bin/insta-game-host
sudo tee /etc/systemd/system/insta-game.service >/dev/null <<'SERVICE'
[Unit]
Description=Shared game host controller
Wants=network-online.target
After=network-online.target
[Service]
Type=simple
EnvironmentFile=-/etc/insta-game.env
EnvironmentFile=-/etc/insta-game-lobby.env
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
sudo systemctl enable insta-game.service
sudo systemctl restart insta-game.service
sudo systemctl is-active insta-game.service
`)
