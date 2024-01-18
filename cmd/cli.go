package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/IrineSistiana/mosdns/v5/mlog"
	"github.com/kardianos/service"
	"github.com/mitchellh/mapstructure"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

type serverFlags struct {
	c         string
	asService bool
}

var rootCmd = &cobra.Command{
	Use: "guber",
}

func init() {
	sf := new(serverFlags)
	startCmd := &cobra.Command{
		Use:   "start [-c config_file]",
		Short: "Start guber main program.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if sf.asService {
				svc, err := service.New(&serverService{f: sf}, svcCfg)
				if err != nil {
					return fmt.Errorf("failed to init service, %w", err)
				}
				return svc.Run()
			}

			m, err := NewServer(sf)
			if err != nil {
				return err
			}

			go func() {
				c := make(chan os.Signal, 1)
				signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
				sig := <-c
				m.Logger().Warn("signal received", zap.Stringer("signal", sig))
				m.sc.SendCloseSignal(nil)
			}()
			return m.GetSafeClose().WaitClosed()
		},
		DisableFlagsInUseLine: true,
		SilenceUsage:          true,
	}
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(runCmd())
	fs := startCmd.Flags()
	fs.StringVarP(&sf.c, "config", "c", "", "config file")
	fs.BoolVar(&sf.asService, "as-service", false, "start as a service")
	_ = fs.MarkHidden("as-service")

	serviceCmd := &cobra.Command{
		Use:   "service",
		Short: "Manage guber as a system service.",
	}
	serviceCmd.PersistentPreRunE = initService
	serviceCmd.AddCommand(
		newSvcInstallCmd(),
		newSvcUninstallCmd(),
		newSvcStartCmd(),
		newSvcStopCmd(),
		newSvcRestartCmd(),
		newSvcStatusCmd(),
	)
	rootCmd.AddCommand(serviceCmd)
}

func AddSubCmd(c *cobra.Command) {
	rootCmd.AddCommand(c)
}

func Run() error {
	return rootCmd.Execute()
}

func NewServer(sf *serverFlags) (*Guber, error) {

	cfg, fileUsed, err := loadConfig(sf.c)
	if err != nil {
		return nil, fmt.Errorf("fail to load config, %w", err)
	}
	mlog.L().Info("main config loaded", zap.String("file", fileUsed))

	return NewGuber(cfg)
}

func loadConfig(filePath string) (*Config, string, error) {
	v := viper.New()

	if len(filePath) > 0 {
		v.SetConfigFile(filePath)
	} else {
		v.SetConfigName("config")
		v.AddConfigPath(".")
	}

	if err := v.ReadInConfig(); err != nil {
		return nil, "", fmt.Errorf("failed to read config: %w", err)
	}

	decoderOpt := func(cfg *mapstructure.DecoderConfig) {
		cfg.ErrorUnused = true
		cfg.TagName = "yaml"
		cfg.WeaklyTypedInput = true
	}

	cfg := new(Config)
	if err := v.Unmarshal(cfg, decoderOpt); err != nil {
		return nil, "", fmt.Errorf("failed to unmarshal config: %w", err)
	}
	return cfg, v.ConfigFileUsed(), nil
}

func runCmd() *cobra.Command {
	sf := new(serverFlags)
	c := &cobra.Command{
		Use:   "run -c config_file",
		Short: "Dry run with the config file.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			mlog.SetLevel(zap.ErrorLevel)
			g, err := NewServer(sf)
			if err != nil {
				return fmt.Errorf("failed to init service, %w", err)
			}
			fmt.Println(g.Status())
			return nil
		},
		DisableFlagsInUseLine: true,
		SilenceUsage:          true,
	}
	fs := c.Flags()
	fs.StringVarP(&sf.c, "config", "c", "", "config file")
	return c
}
