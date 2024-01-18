package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/IrineSistiana/mosdns/v5/mlog"
	"github.com/IrineSistiana/mosdns/v5/pkg/safe_close"
	"github.com/txn2/txeh"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Guber struct {
	sc *safe_close.SafeClose

	nacos  *[]NacosClient
	config *Config
	hosts  *txeh.Hosts
}

func NewGuber(cfg *Config) (*Guber, error) {
	// Init logger.
	l, err := zapcore.ParseLevel(cfg.Log.Level)
	if err != nil {
		return nil, fmt.Errorf("invalid log level: %w", err)
	}
	mlog.SetLevel(l)

	hosts, err := txeh.NewHostsDefault()
	if err != nil {
		mlog.L().Error("failed to init hosts", zap.Error(err))
		return nil, err
	}

	backupPath := hosts.HostsConfig.ReadFilePath + ".bak"
	err = hosts.SaveAs(backupPath)
	if err != nil {
		mlog.L().Warn("failed to backup hosts", zap.Error(err), zap.String("path", backupPath))
	}

	var ncs []NacosClient
	sc := safe_close.NewSafeClose()
	g := &Guber{
		config: cfg,
		sc:     sc,
		hosts:  hosts,
	}

	// Init nacos clients.
	for _, app := range cfg.App {
		nc := NewNacosClient(&app.Nacos)

		err := nc.Run(g)
		if err != nil {
			return nil, err
		}
		ncs = append(ncs, *nc)
		g.watch(nc, app.Names, app.Env)
	}
	g.nacos = &ncs

	g.sc.Attach(func(done func(), closeSignal <-chan struct{}) {
		defer done()
		<-closeSignal
		g.restore(backupPath, hosts)

	})

	mlog.L().Info("all nacos clients are loaded")

	return g, nil
}

func (g *Guber) restore(backupPath string, hosts *txeh.Hosts) {
	bak, err := txeh.NewHosts(&txeh.HostsConfig{ReadFilePath: backupPath, WriteFilePath: hosts.HostsConfig.WriteFilePath})
	if err != nil {
		mlog.L().Error("failed to read from backup hosts", zap.Error(err), zap.String("read", backupPath))
		return
	}
	mlog.L().Debug("restore hosts", zap.String("read", backupPath), zap.String("write", bak.HostsConfig.WriteFilePath))
	err = bak.Save()
	if err != nil {
		mlog.L().Error("failed to restore hosts", zap.Error(err), zap.String("read", backupPath), zap.String("write", hosts.HostsConfig.WriteFilePath))
		return
	}
	mlog.L().Debug("delete", zap.String("path", backupPath))
	err = os.Remove(backupPath)
	if err != nil {
		mlog.L().Error("failed to remove backup hosts", zap.Error(err), zap.String("path", backupPath))
	}
}

// Status prints the information of the nacos and application.
func (g *Guber) Status() string {
	var output string
	for _, app := range g.config.App {
		nc := NewNacosClient(&app.Nacos)
		err := nc.Run(g)
		if err != nil {
			output += fmt.Sprintf("nacos: %s,error \n", nc.NacosConfig.Addr)
		} else {
			output += fmt.Sprintf("nacos: %s\n", nc.NacosConfig.Addr)
		}
		for _, app := range g.config.App {
			for _, name := range app.Names {
				_, h, err := nc.GetNacosService(name)
				if err == nil {
					output += fmt.Sprintf("  %s: %s\n", name, h)
				} else {
					output += fmt.Sprintf("  %s: error\n", name)
				}
			}
		}
	}

	return output
}

func (g *Guber) GetSafeClose() *safe_close.SafeClose {
	return g.sc
}

// CloseWithErr is a shortcut for m.sc.SendCloseSignal
func (g *Guber) CloseWithErr(err error) {
	g.sc.SendCloseSignal(err)
}

// Logger returns a non-nil logger.
func (g *Guber) Logger() *zap.Logger {
	return mlog.L()
}

func (g *Guber) updateHosts(env string, app string, h []string) {
	host := fmt.Sprintf("%s.%s", app, env)
	mlog.L().Info("update hosts", zap.String("host", host), zap.Strings("hosts", h))
	g.hosts.RemoveHost(host)
	for _, ip := range h {
		mlog.L().Debug("add host", zap.String("host", host), zap.String("ip", ip))
		g.hosts.AddHost(ip, host)
	}
	err := g.hosts.Save()
	if err != nil {
		mlog.L().Error("failed to save hosts", zap.Error(err))
	}
}

func (g *Guber) watch(nc *NacosClient, apps []string, env string) {
	ticker := time.NewTicker(10 * time.Second)
	go func() {
		select {
		case <-g.GetSafeClose().ReceiveCloseSignal():
			{
				ticker.Stop()
			}
		case <-ticker.C:
			{
				for _, app := range apps {
					app, h, err := nc.GetNacosService(app)
					if err == nil {
						g.updateHosts(env, app, h)
					}
				}
			}
		}

	}()
}
