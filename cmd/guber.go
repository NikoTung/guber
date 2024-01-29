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

	nacos  map[string]*NacosClient
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

	var ncs = make(map[string]*NacosClient, len(cfg.App))
	sc := safe_close.NewSafeClose()
	g := &Guber{
		config: cfg,
		sc:     sc,
		hosts:  hosts,
	}

	// Init nacos clients.
	for _, app := range cfg.App {
		n := app
		nc := NewNacosClient(&n.Nacos, app.Env)
		mlog.L().Debug("Checking nacos connection", zap.String("env", n.Env), zap.String("nacos", nc.NacosConfig.Addr))
		err := nc.Run(g)
		if err != nil {
			return nil, err
		}
		ncs[n.Env] = nc
	}
	g.nacos = ncs
	g.watch()

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
		nc := NewNacosClient(&app.Nacos, app.Env)
		err := nc.Run(g)
		if err != nil {
			output += fmt.Sprintf("nacos: %s,error \n", nc.NacosConfig.Addr)
		} else {
			output += fmt.Sprintf("nacos: %s\n", nc.NacosConfig.Addr)
		}
		for _, name := range app.Names {
			_, h, err := nc.GetNacosService(name, app.Keep)
			if err == nil {
				output += fmt.Sprintf("  %s: %s\n", name, h)
			} else {
				mlog.L().Error("failed to get nacos service", zap.String("name", name), zap.Error(err))
				output += fmt.Sprintf("  %s: error\n", name)
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

func (g *Guber) updateHosts(app string, h []string) {
	mlog.L().Info("update hosts", zap.String("host", app), zap.Strings("hosts", h))
	g.hosts.RemoveHost(app)
	for _, ip := range h {
		mlog.L().Debug("add host", zap.String("host", app), zap.String("ip", ip))
		g.hosts.AddHost(ip, app)
	}
	err := g.hosts.Save()
	if err != nil {
		mlog.L().Error("failed to save hosts", zap.Error(err))
	}
}

func (g *Guber) watch() {
	ticker := time.NewTicker(10 * time.Second)
	g.sc.Attach(func(done func(), closeSignal <-chan struct{}) {
		defer done()
		<-closeSignal
		ticker.Stop()
	})
	go func() {
		for range ticker.C {
			for _, app := range g.config.App {
				nc := g.nacos[app.Env]
				mlog.L().Debug("watch", zap.String("env", app.Env), zap.String("nacos", nc.NacosConfig.Addr), zap.Strings("apps", app.Names))
				for _, name := range app.Names {
					app, h, err := nc.GetNacosService(name, app.Keep)
					if err == nil {
						g.updateHosts(app, h)
					}
				}
			}
		}

	}()
}
