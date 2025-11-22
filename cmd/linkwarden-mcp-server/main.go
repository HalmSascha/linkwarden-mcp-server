package main

import (
    "context"
    "fmt"
    "io"
    stdlog "log"
    "log/slog"
    "net/http"
    "os"
    "os/signal"
    "strings"
    "syscall"

    "github.com/irfansofyana/linkwarden-mcp-server/pkg/contextkey"
    "github.com/irfansofyana/linkwarden-mcp-server/pkg/linkwarden"
    "github.com/irfansofyana/linkwarden-mcp-server/pkg/linkwardenmcp"
    "github.com/irfansofyana/linkwarden-mcp-server/pkg/log"
    "github.com/irfansofyana/linkwarden-mcp-server/pkg/mcpgo"
    "github.com/irfansofyana/linkwarden-mcp-server/pkg/observability"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	version = "version"
	commit  = "commit"
	date    = "date"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:     "server",
	Short:   "Linkwarden MCP Server",
	Version: fmt.Sprintf("%s\ncommit %s\ndate %s", version, commit, date),
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName("linkwarden-mcp-server")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}

// stdioCmd starts the mcp server in stdio transport mode
var stdioCmd = &cobra.Command{
	Use:   "stdio",
	Short: "start the stdio server",
	Run: func(cmd *cobra.Command, args []string) {
		logPath := viper.GetString("log_file")

		config := log.NewConfig(
			log.WithMode(log.ModeStdio),
			log.WithLogLevel(slog.LevelInfo),
			log.WithLogPath(logPath),
		)

		ctx, logger := log.New(context.Background(), config)

		// Create observability with logging
		obs := observability.New(
			observability.WithLogging(logger),
		)

		token := viper.GetString("token")
		baseUrl := viper.GetString("base_url")

		client, err := linkwarden.NewClientWithResponses(baseUrl, linkwarden.WithRequestEditorFn(
			func(ctx context.Context, req *http.Request) error {
				req.Header.Set("Authorization", "Bearer "+token)
				return nil
			},
		))
		if err != nil {
			obs.Logger.Errorf(ctx,
				"error running stdio server", "error", err)
			stdlog.Fatalf("failed to run stdio server: %v", err)
		}

		// Get toolsets to enable from config
		enabledToolsets := viper.GetStringSlice("toolsets")

		// Get read-only mode from config
		readOnly := viper.GetBool("read_only")

		if err := runStdioServer(ctx, obs, client, enabledToolsets, readOnly); err != nil {
			obs.Logger.Errorf(ctx,
				"error running stdio server", "error", err)
			stdlog.Fatalf("failed to run stdio server: %v", err)
		}
	},
}

// httpCmd starts the mcp server in StreamableHTTP transport mode
var httpCmd = &cobra.Command{
    Use:   "http",
    Short: "start the http server",
    Run: func(cmd *cobra.Command, args []string) {
        logPath := viper.GetString("log_file")
        addr := viper.GetString("http_addr")

        config := log.NewConfig(
            log.WithMode(log.ModeStdio),
            log.WithLogLevel(slog.LevelInfo),
            log.WithLogPath(logPath),
        )

        ctx, logger := log.New(context.Background(), config)

        obs := observability.New(
            observability.WithLogging(logger),
        )

        token := viper.GetString("token")
        baseUrl := viper.GetString("base_url")

        client, err := linkwarden.NewClientWithResponses(baseUrl, linkwarden.WithRequestEditorFn(
            func(ctx context.Context, req *http.Request) error {
                if token != "" {
                    req.Header.Set("Authorization", "Bearer "+token)
                }
                return nil
            },
        ))
        if err != nil {
            obs.Logger.Errorf(ctx, "error creating default client", "error", err)
        }

        enabledToolsets := viper.GetStringSlice("toolsets")
        readOnly := viper.GetBool("read_only")

        srv, err := linkwardenmcp.NewLinkwardenMcpServer(obs, client, enabledToolsets, readOnly)
        if err != nil {
            obs.Logger.Errorf(ctx, "error creating server", "error", err)
            stdlog.Fatalf("failed to create server: %v", err)
        }

        httpSrv, err := mcpgo.NewHTTPServer(srv)
        if err != nil {
            obs.Logger.Errorf(ctx, "error creating http server", "error", err)
            stdlog.Fatalf("failed to create http server: %v", err)
        }

        mux := http.NewServeMux()

        mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
            w.WriteHeader(http.StatusOK)
            _, _ = w.Write([]byte("ok"))
        })

        mcpHandler := httpSrv.Handler()
        mux.Handle("/mcp", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            lwToken := r.Header.Get("X-Linkwarden-Token")
            lwBase := r.Header.Get("X-Linkwarden-BaseURL")
            toolsetsHeader := r.Header.Get("X-Toolsets")
            readOnlyHeader := r.Header.Get("X-Read-Only")

            if lwToken == "" || lwBase == "" {
                http.Error(w, "missing headers", http.StatusUnauthorized)
                return
            }

            reqClient, cerr := linkwarden.NewClientWithResponses(lwBase, linkwarden.WithRequestEditorFn(
                func(ctx context.Context, req *http.Request) error {
                    req.Header.Set("Authorization", "Bearer "+lwToken)
                    return nil
                },
            ))
            if cerr != nil {
                http.Error(w, "invalid base url", http.StatusBadRequest)
                return
            }

            newCtx := contextkey.WithClient(r.Context(), reqClient)

            if toolsetsHeader != "" {
                parts := strings.Split(toolsetsHeader, ",")
                var normalized []string
                for _, p := range parts {
                    normalized = append(normalized, strings.ToLower(strings.TrimSpace(p)))
                }
                newCtx = contextkey.WithAllowedToolsets(newCtx, normalized)
            }

            if strings.EqualFold(strings.TrimSpace(readOnlyHeader), "true") {
                newCtx = contextkey.WithReadOnly(newCtx, true)
            }

            mcpHandler.ServeHTTP(w, r.WithContext(newCtx))
        }))

        _, _ = fmt.Fprintf(os.Stderr, "Linkwarden MCP Server running on http %s\n", addr)
        if err := http.ListenAndServe(addr, mux); err != nil {
            obs.Logger.Errorf(ctx, "server error", "error", err)
            stdlog.Fatalf("failed to start http server: %v", err)
        }
    },
}

func runStdioServer(
	ctx context.Context,
	obs *observability.Observability,
	client *linkwarden.ClientWithResponses,
	enabledToolsets []string,
	readOnly bool,
) error {
	ctx, stop := signal.NotifyContext(
		ctx,
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	srv, err := linkwardenmcp.NewLinkwardenMcpServer(obs, client, enabledToolsets, readOnly)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	stdioSrv, err := mcpgo.NewStdioServer(srv)
	if err != nil {
		return fmt.Errorf("failed to create stdio server: %w", err)
	}

	in, out := io.Reader(os.Stdin), io.Writer(os.Stdout)
	errC := make(chan error, 1)
	go func() {
		obs.Logger.Infof(ctx, "starting server")
		errC <- stdioSrv.Listen(ctx, in, out)
	}()

	_, _ = fmt.Fprintf(
		os.Stderr,
		"Linkwarden MCP Server running on stdio\n",
	)

	// Wait for shutdown signal
	select {
	case <-ctx.Done():
		obs.Logger.Infof(ctx, "shutting down server...")
		return nil
	case err := <-errC:
		if err != nil {
			obs.Logger.Errorf(ctx, "server error", "error", err)
			return err
		}
		return nil
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringP("base-url", "b", "", "your linkwarden base url")
	rootCmd.PersistentFlags().StringP("token", "s", "", "your linkwarden secret / token")
	rootCmd.PersistentFlags().StringP("log-file", "l", "", "path to the log file")
	rootCmd.PersistentFlags().StringSliceP("toolsets", "t", []string{}, "comma-separated list of toolsets to enable")
    rootCmd.PersistentFlags().Bool("read-only", false, "run server in read-only mode")

	_ = viper.BindPFlag("base_url", rootCmd.PersistentFlags().Lookup("base-url"))
	_ = viper.BindPFlag("token", rootCmd.PersistentFlags().Lookup("token"))
	_ = viper.BindPFlag("log_file", rootCmd.PersistentFlags().Lookup("log-file"))
	_ = viper.BindPFlag("toolsets", rootCmd.PersistentFlags().Lookup("toolsets"))
	_ = viper.BindPFlag("read_only", rootCmd.PersistentFlags().Lookup("read-only"))

    _ = viper.BindEnv("base_url", "LINKWARDEN_BASE_URL")
    _ = viper.BindEnv("token", "LINKWARDEN_TOKEN")

	// Enable environment variable reading
	viper.AutomaticEnv()

    // subcommands
    rootCmd.AddCommand(stdioCmd)
    rootCmd.AddCommand(httpCmd)

    httpCmd.Flags().String("addr", ":8080", "address for http server")
    _ = viper.BindPFlag("http_addr", httpCmd.Flags().Lookup("addr"))
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
