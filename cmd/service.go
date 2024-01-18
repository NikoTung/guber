package cmd

import (
	"fmt"
	"time"

	"github.com/IrineSistiana/mosdns/v5/mlog"
	"github.com/kardianos/service"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var (
	// initialized by "service" sub command
	svc    service.Service
	svcCfg = &service.Config{
		Name:        "guber",
		DisplayName: "guber",
		Description: "A Nacos Application-IP-Hosts register service.",
	}
)

type serverService struct {
	f *serverFlags
	g *Guber
}

func (ss *serverService) Start(s service.Service) error {
	mlog.L().Info("starting service", zap.String("platform", s.Platform()))
	g, err := NewServer(ss.f)
	if err != nil {
		return err
	}
	ss.g = g
	go func() {
		err := g.GetSafeClose().WaitClosed()
		if err != nil {
			g.Logger().Fatal("server exited", zap.Error(err))
		} else {
			g.Logger().Info("server exited")
		}
	}()
	return nil
}

func (ss *serverService) Stop(_ service.Service) error {
	ss.g.Logger().Info("service is shutting down")
	ss.g.GetSafeClose().SendCloseSignal(nil)
	return ss.g.GetSafeClose().WaitClosed()
}

func initService(_ *cobra.Command, _ []string) error {
	s, err := service.New(&serverService{}, svcCfg)
	if err != nil {
		return fmt.Errorf("cannot init service, %w", err)
	}
	svc = s
	return nil
}

func newSvcInstallCmd() *cobra.Command {
	sf := new(serverFlags)
	c := &cobra.Command{
		Use:   "install [-c config_file]",
		Short: "Install guber as a system service.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(sf.c) <= 0 {
				return fmt.Errorf("config file is required")
			}
			svcCfg.Arguments = []string{"start", "--as-service", "-c", sf.c}
			return svc.Install()
		},
		DisableFlagsInUseLine: true,
		SilenceUsage:          true,
	}
	c.Flags().StringVarP(&sf.c, "config", "c", "", "config path")
	return c
}

func newSvcUninstallCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "uninstall",
		Short: "Uninstall guber from system service.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return svc.Uninstall()
		},
		DisableFlagsInUseLine: true,
		SilenceUsage:          true,
	}
	return c
}

func newSvcStartCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "start",
		Short: "Start guber system service.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := svc.Start(); err != nil {
				return err
			}

			mlog.S().Info("service is starting")
			time.Sleep(time.Second)
			s, err := svc.Status()
			if err != nil {
				mlog.S().Warn("cannot get service status, %w", err)
			} else {
				switch s {
				case service.StatusRunning:
					mlog.S().Info("service is running")
				case service.StatusStopped:
					mlog.S().Error("service is stopped, check guber and system service log for more info")
				default:
					mlog.S().Warn("cannot get service status, system may not support this operation")
				}
			}

			return nil
		},
		DisableFlagsInUseLine: true,
		SilenceUsage:          true,
	}
	return c
}

func newSvcStopCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "stop",
		Short: "Stop guber system service.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return svc.Stop()
		},
		DisableFlagsInUseLine: true,
		SilenceUsage:          true,
	}
	return c
}

func newSvcRestartCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "restart",
		Short: "Restart guber system service.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return svc.Restart()
		},
		DisableFlagsInUseLine: true,
		SilenceUsage:          true,
	}
	return c
}

func newSvcStatusCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "status",
		Short: "Status of guber system service.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := svc.Status()
			if err != nil {
				return fmt.Errorf("cannot get service status, %w", err)
			}
			var out string
			switch s {
			case service.StatusRunning:
				out = "running"
			case service.StatusStopped:
				out = "stopped"
			case service.StatusUnknown:
				out = "unknown"
			}
			println(out)
			return nil
		},
		DisableFlagsInUseLine: true,
		SilenceUsage:          true,
	}
	return c
}
