package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"dockerman/internal/docker"
	"dockerman/internal/server"
	"dockerman/internal/store"

	"github.com/spf13/cobra"
)

var dbPath string

func defaultDBPath() string {
	if p := os.Getenv("CONTAINER_DB"); p != "" {
		return p
	}
	return "/var/lib/dockerman/containers.json"
}

func getDockerClient() (*docker.Client, error) {
	return docker.NewClient()
}

func resolveIDs(dockerCli *docker.Client, jsonStore *store.JSONStore, arg string) ([]string, error) {
	if arg == "all" {
		containers, err := jsonStore.Load()
		if err != nil {
			return nil, fmt.Errorf("load store: %w", err)
		}
		ids := make([]string, len(containers))
		for i, c := range containers {
			ids[i] = c.ID
		}
		return ids, nil
	}

	// Try exact match via Docker inspect first
	if _, err := dockerCli.Inspect(context.Background(), arg); err == nil {
		return []string{arg}, nil
	}

	// Fall back to store lookup
	info, err := jsonStore.GetByID(arg)
	if err != nil {
		return nil, fmt.Errorf("container %q not found", arg)
	}
	return []string{info.ID}, nil
}

func scanCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "scan",
		Short: "Scan all Docker containers and save to database",
		RunE: func(cmd *cobra.Command, args []string) error {
			dockerCli, err := getDockerClient()
			if err != nil {
				return err
			}
			defer dockerCli.Close()

			containers, err := dockerCli.ScanAll(context.Background())
			if err != nil {
				return fmt.Errorf("scan containers: %w", err)
			}

			jsonStore := store.NewJSONStore(dbPath)
			if err := jsonStore.Save(containers); err != nil {
				return fmt.Errorf("save containers: %w", err)
			}

			fmt.Printf("Scanned %d container(s) and saved to %s\n", len(containers), dbPath)
			return nil
		},
	}
}

func listCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List scanned containers from database",
		RunE: func(cmd *cobra.Command, args []string) error {
			jsonStore := store.NewJSONStore(dbPath)
			containers, err := jsonStore.Load()
			if err != nil {
				return fmt.Errorf("load containers: %w", err)
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
			fmt.Fprintln(w, "ID\tNAME\tIMAGE\tSTATUS\tPORTS\tCREATED")
			for _, c := range containers {
				ports := strings.Join(c.Ports, ",")
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", c.ID, c.Name, c.Image, c.Status, ports, c.CreatedAt)
			}
			w.Flush()
			return nil
		},
	}
}

func startCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start <id|all>",
		Short: "Start a container",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dockerCli, err := getDockerClient()
			if err != nil {
				return err
			}
			defer dockerCli.Close()

			jsonStore := store.NewJSONStore(dbPath)
			ids, err := resolveIDs(dockerCli, jsonStore, args[0])
			if err != nil {
				return err
			}

			ctx := context.Background()
			for _, id := range ids {
				if err := dockerCli.Start(ctx, id); err != nil {
					return fmt.Errorf("start %s: %w", id, err)
				}
				fmt.Printf("Started container %s\n", id)
			}
			return nil
		},
	}
}

func stopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop <id|all>",
		Short: "Stop a container",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dockerCli, err := getDockerClient()
			if err != nil {
				return err
			}
			defer dockerCli.Close()

			jsonStore := store.NewJSONStore(dbPath)
			ids, err := resolveIDs(dockerCli, jsonStore, args[0])
			if err != nil {
				return err
			}

			ctx := context.Background()
			for _, id := range ids {
				if err := dockerCli.Stop(ctx, id); err != nil {
					return fmt.Errorf("stop %s: %w", id, err)
				}
				fmt.Printf("Stopped container %s\n", id)
			}
			return nil
		},
	}
}

func rmCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "rm <id|all>",
		Short: "Remove a container",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dockerCli, err := getDockerClient()
			if err != nil {
				return err
			}
			defer dockerCli.Close()

			jsonStore := store.NewJSONStore(dbPath)
			ids, err := resolveIDs(dockerCli, jsonStore, args[0])
			if err != nil {
				return err
			}

			ctx := context.Background()
			for _, id := range ids {
				if err := dockerCli.Remove(ctx, id, force); err != nil {
					return fmt.Errorf("remove %s: %w", id, err)
				}
				fmt.Printf("Removed container %s\n", id)
			}
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Force remove running container")
	return cmd
}

func execCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "exec <id> <command...>",
		Short: "Execute a command in a container",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dockerCli, err := getDockerClient()
			if err != nil {
				return err
			}
			defer dockerCli.Close()

			id := args[0]
			command := args[1:]

			output, err := dockerCli.Exec(context.Background(), id, command)
			if err != nil {
				return fmt.Errorf("exec %s: %w", id, err)
			}

			fmt.Print(output)
			return nil
		},
	}
}

func inspectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "inspect <id>",
		Short: "Inspect a container",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dockerCli, err := getDockerClient()
			if err != nil {
				return err
			}
			defer dockerCli.Close()

			info, err := dockerCli.Inspect(context.Background(), args[0])
			if err != nil {
				return fmt.Errorf("inspect container: %w", err)
			}

			fmt.Printf("ID: %s\n", info.ID)
			fmt.Printf("Name: %s\n", strings.TrimPrefix(info.Name, "/"))
			if info.State != nil {
				fmt.Printf("Status: %s\n", info.State.Status)
				fmt.Printf("Running: %t\n", info.State.Running)
				fmt.Printf("Started: %s\n", info.State.StartedAt)
				fmt.Printf("Finished: %s\n", info.State.FinishedAt)
			}
			if info.Config != nil {
				fmt.Printf("Image: %s\n", info.Config.Image)
			}
			return nil
		},
	}
}

func serveCmd() *cobra.Command {
	var port int
	var host string

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the HTTP API server",
		RunE: func(cmd *cobra.Command, args []string) error {
			srv := server.NewServer(host, port, dbPath)
			return srv.Start()
		},
	}

	cmd.Flags().IntVarP(&port, "port", "p", 8080, "Server port")
	cmd.Flags().StringVar(&host, "host", "0.0.0.0", "Server host")
	return cmd
}

func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "dockerman",
		Short: "Docker Container Manager",
	}

	root.PersistentFlags().StringVar(&dbPath, "db", defaultDBPath(), "Path to container database file")

	root.AddCommand(
		scanCmd(),
		listCmd(),
		startCmd(),
		stopCmd(),
		rmCmd(),
		execCmd(),
		inspectCmd(),
		serveCmd(),
	)

	return root
}
