package cli

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

const (
	defaultBinPath  = "/usr/local/bin/dockerman"
	unitPath        = "/etc/systemd/system/dockerman.service"
	serviceName     = "dockerman"
	authTokenPath   = "/var/lib/dockerman/auth"
)

func installCmd() *cobra.Command {
	var port int
	var force bool
	var noEnable bool

	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install dockerman systemd service",
		RunE: func(cmd *cobra.Command, args []string) error {
			if os.Geteuid() != 0 {
				return fmt.Errorf("root privileges required; run with sudo")
			}

			// Stop existing service before overwriting binary
			if _, err := os.Stat(unitPath); err == nil {
				if force {
					_ = runCommand("systemctl", "stop", serviceName)
					fmt.Println("Stopped running dockerman service")
				} else {
					fmt.Print("Overwrite existing installation? (y/N): ")
					reader := bufio.NewReader(os.Stdin)
					resp, err := reader.ReadString('\n')
					if err != nil {
						return fmt.Errorf("read input: %w", err)
					}
					if strings.ToLower(strings.TrimSpace(resp)) != "y" {
						fmt.Println("Skipping installation.")
						return nil
					}
					_ = runCommand("systemctl", "stop", serviceName)
				}
			}

			src, err := os.Executable()
			if err != nil {
				return fmt.Errorf("determine executable path: %w", err)
			}

			if err := copyFile(src, defaultBinPath); err != nil {
				return fmt.Errorf("install binary to %s: %w", defaultBinPath, err)
			}
			fmt.Printf("Installed binary to %s\n", defaultBinPath)

			if err := os.MkdirAll("/var/lib/dockerman", 0777); err != nil {
				return fmt.Errorf("create data directory: %w", err)
			}
			if err := os.Chmod("/var/lib/dockerman", 0777); err != nil {
				return fmt.Errorf("chmod data directory: %w", err)
			}
			fmt.Println("Set up data directory /var/lib/dockerman")

			token, err := readOrGenerateToken()
			if err != nil {
				return fmt.Errorf("auth token: %w", err)
			}

			unit := fmt.Sprintf(`[Unit]
Description=Docker Container Manager
After=docker.service network.target
Requires=docker.service

[Service]
Type=simple
Environment=DOCKERMAN_AUTH_TOKEN=%s
ExecStart=%s serve --port %d --db /var/lib/dockerman/containers.json
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
`, token, defaultBinPath, port)

			if err := os.WriteFile(unitPath, []byte(unit), 0644); err != nil {
				return fmt.Errorf("write unit file: %w", err)
			}
			fmt.Println("Wrote", unitPath)

			if err := runCommand("systemctl", "daemon-reload"); err != nil {
				return fmt.Errorf("daemon-reload: %w", err)
			}
			fmt.Println("Ran systemctl daemon-reload")

			if !noEnable {
				if err := runCommand("systemctl", "enable", serviceName); err != nil {
					return fmt.Errorf("enable %s: %w", serviceName, err)
				}
				fmt.Println("Enabled dockerman service")
			}

			if err := runCommand("systemctl", "start", serviceName); err != nil {
				return fmt.Errorf("start %s: %w", serviceName, err)
			}
			fmt.Println("Started dockerman service")

			fmt.Println("dockerman systemd service installed successfully.")
			return nil
		},
	}

	cmd.Flags().IntVar(&port, "port", 5001, "API server port")
	cmd.Flags().BoolVar(&force, "force", false, "Skip overwrite prompt")
	cmd.Flags().BoolVar(&noEnable, "no-enable", false, "Install unit but do not enable")

	return cmd
}

func uninstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall",
		Short: "Uninstall dockerman systemd service",
		RunE: func(cmd *cobra.Command, args []string) error {
			if os.Geteuid() != 0 {
				return fmt.Errorf("root privileges required; run with sudo")
			}

			_ = runCommand("systemctl", "stop", serviceName)
			_ = runCommand("systemctl", "disable", serviceName)
			fmt.Println("Stopped and disabled dockerman service")

			if err := os.Remove(unitPath); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("remove unit file: %w", err)
			}
			fmt.Println("Removed", unitPath)

			if err := runCommand("systemctl", "daemon-reload"); err != nil {
				return fmt.Errorf("daemon-reload: %w", err)
			}
			fmt.Println("Ran systemctl daemon-reload")

			if err := os.Remove(defaultBinPath); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("remove binary: %w", err)
			}
			fmt.Println("Removed", defaultBinPath)

			_ = os.Remove(authTokenPath)

			fmt.Println("dockerman systemd service uninstalled successfully.")
			return nil
		},
	}
}

func readOrGenerateToken() (string, error) {
	if data, err := os.ReadFile(authTokenPath); err == nil {
		token := strings.TrimSpace(string(data))
		if token != "" {
			fmt.Printf("Using existing auth token from %s\n", authTokenPath)
			return token, nil
		}
	}
	token, err := generateToken()
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(authTokenPath, []byte(token+"\n"), 0644); err != nil {
		return "", fmt.Errorf("write auth token: %w", err)
	}
	fmt.Printf("Wrote auth token to %s\n", authTokenPath)
	return token, nil
}

func generateToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
