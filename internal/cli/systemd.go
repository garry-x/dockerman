package cli

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

func installSystemdCmd() *cobra.Command {
	var bin string
	var port int
	var force bool
	var noEnable bool

	cmd := &cobra.Command{
		Use:   "install-systemd",
		Short: "Install dockerman as a systemd service",
		RunE: func(cmd *cobra.Command, args []string) error {
			if os.Geteuid() != 0 {
				return fmt.Errorf("root privileges required; run with sudo")
			}

			if bin == "" {
				exe, err := os.Executable()
				if err != nil {
					return fmt.Errorf("determine executable path: %w", err)
				}
				bin = exe
			}

			unit := fmt.Sprintf(`[Unit]
Description=Docker Container Manager
After=docker.service network.target
Requires=docker.service

[Service]
Type=simple
ExecStart=%s serve --port %d --db %s
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
`, bin, port, dbPath)

			unitPath := "/etc/systemd/system/dockerman.service"

			// Check if file exists and prompt for overwrite
			if _, err := os.Stat(unitPath); err == nil && !force {
				fmt.Print("Overwrite? (y/N): ")
				reader := bufio.NewReader(os.Stdin)
				resp, err := reader.ReadString('\n')
				if err != nil {
					return fmt.Errorf("read input: %w", err)
				}
				resp = strings.TrimSpace(resp)
				if strings.ToLower(resp) != "y" {
					fmt.Println("Skipping installation.")
					return nil
				}
			}

			if err := os.WriteFile(unitPath, []byte(unit), 0644); err != nil {
				return fmt.Errorf("write unit file: %w", err)
			}
			fmt.Println("Wrote", unitPath)

			if err := runCommand("systemctl", "daemon-reload"); err != nil {
				return fmt.Errorf("daemon-reload: %w", err)
			}
			fmt.Println("Ran systemctl daemon-reload")

			if !noEnable {
				if err := runCommand("systemctl", "enable", "dockerman"); err != nil {
					return fmt.Errorf("enable dockerman: %w", err)
				}
				fmt.Println("Enabled dockerman service")
			}

			fmt.Println("dockerman systemd service installed successfully.")
			return nil
		},
	}

	cmd.Flags().StringVar(&bin, "bin", "", "Path to dockerman binary (default: uses os.Executable())")
	cmd.Flags().IntVar(&port, "port", 8080, "API server port")
	cmd.Flags().BoolVar(&force, "force", false, "Skip overwrite prompt")
	cmd.Flags().BoolVar(&noEnable, "no-enable", false, "Install unit but do not enable")

	return cmd
}

func uninstallSystemdCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall-systemd",
		Short: "Uninstall dockerman systemd service",
		RunE: func(cmd *cobra.Command, args []string) error {
			if os.Geteuid() != 0 {
				return fmt.Errorf("root privileges required; run with sudo")
			}

			unitPath := "/etc/systemd/system/dockerman.service"

			// Stop and disable (ignore errors if not running/enabled)
			_ = runCommand("systemctl", "stop", "dockerman")
			_ = runCommand("systemctl", "disable", "dockerman")
			fmt.Println("Stopped and disabled dockerman service")

			// Remove unit file
			if err := os.Remove(unitPath); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("remove unit file: %w", err)
			}
			fmt.Println("Removed", unitPath)

			if err := runCommand("systemctl", "daemon-reload"); err != nil {
				return fmt.Errorf("daemon-reload: %w", err)
			}
			fmt.Println("Ran systemctl daemon-reload")

			fmt.Println("dockerman systemd service uninstalled successfully.")
			return nil
		},
	}
}

func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
