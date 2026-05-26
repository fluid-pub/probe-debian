package probe

import (
	"context"
	"log"
	"sync"
	"time"

	"fluid/probes/debian/internal/collect"
	"fluid/probes/debian/internal/config"
	"fluid/probes/debian/internal/shipper"
)

type Probe struct {
	cfg    *config.Runtime
	client *shipper.Client
	spool  *shipper.Spool
	wg     sync.WaitGroup
}

func New(cfg *config.Runtime) (*Probe, error) {
	return &Probe{
		cfg: cfg,
		client: shipper.New(
			cfg.Raw.Controlplane.BaseURL,
			cfg.Raw.Auth.OrganizationUUID,
			cfg.Raw.Auth.Token,
			cfg.RequestTimeout,
		),
		spool: shipper.NewSpool(cfg.Raw.Spool.Path, cfg.Raw.Spool.MaxEntries),
	}, nil
}

func (p *Probe) Start(ctx context.Context) error {
	if err := p.client.Register(ctx); err != nil {
		log.Printf("register warning: %v", err)
	}

	p.wg.Add(5)
	go p.runSystemLoop(ctx)
	go p.runFilesLoop(ctx)
	go p.runAptLoop(ctx)
	go p.runInstalledPackagesLoop(ctx)
	go p.runServicesLoop(ctx)

	<-ctx.Done()
	p.wg.Wait()
	return nil
}

func (p *Probe) Stop(_ context.Context) error {
	return nil
}

func (p *Probe) runSystemLoop(ctx context.Context) {
	defer p.wg.Done()
	ticker := time.NewTicker(p.cfg.SystemInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			snap, err := collect.CollectSystem(ctx)
			if err != nil {
				log.Printf("system collect failed: %v", err)
				continue
			}
			payload := p.baseStatePayload(p.systemEntities(snap))
			p.sendWithSpool(ctx, payload)
			_ = p.client.Ping(ctx)
		}
	}
}

func (p *Probe) runFilesLoop(ctx context.Context) {
	defer p.wg.Done()
	ticker := time.NewTicker(p.cfg.FilesInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			rows := make([]interface{}, 0, len(p.cfg.Raw.Files)+len(p.cfg.Raw.Directories))
			for _, rule := range p.cfg.Raw.Files {
				check := collect.CollectFile(rule.Path, p.cfg.Raw.Collection.MaxHashFileSize)
				rows = append(rows, check)
			}
			for _, rule := range p.cfg.Raw.Directories {
				check := collect.CollectDirectory(rule.Path, rule.Recursive, p.cfg.Raw.Collection.MaxHashFileSize)
				rows = append(rows, check)
			}
			payload := p.baseStatePayload(map[string]interface{}{
				"debian_file_checks": rows,
			})
			p.sendWithSpool(ctx, payload)
		}
	}
}

func (p *Probe) runAptLoop(ctx context.Context) {
	defer p.wg.Done()
	ticker := time.NewTicker(p.cfg.APTInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			snap := collect.CollectAPT(ctx, p.cfg.Raw.Collection.MaxPackageItems)
			payload := p.baseStatePayload(map[string]interface{}{
				"debian_package_updates": []interface{}{snap},
			})
			p.sendWithSpool(ctx, payload)
		}
	}
}

func (p *Probe) runInstalledPackagesLoop(ctx context.Context) {
	defer p.wg.Done()
	p.pushInstalledPackagesSnapshot(ctx)
	ticker := time.NewTicker(p.cfg.InstalledPackagesInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.pushInstalledPackagesSnapshot(ctx)
		}
	}
}

func (p *Probe) pushInstalledPackagesSnapshot(ctx context.Context) {
	rows, err := collect.CollectInstalledPackages(ctx)
	if err != nil {
		log.Printf("installed packages collect failed: %v", err)
		return
	}
	out := make([]interface{}, len(rows))
	for i := range rows {
		out[i] = rows[i]
	}
	payload := p.baseStatePayload(map[string]interface{}{
		"debian_installed_packages": out,
	})
	p.sendWithSpool(ctx, payload)
}

func (p *Probe) runServicesLoop(ctx context.Context) {
	defer p.wg.Done()
	ticker := time.NewTicker(p.cfg.ServicesInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			rows, err := collect.CollectEnabledServices(ctx)
			if err != nil {
				log.Printf("enabled services collect failed: %v", err)
				continue
			}
			out := make([]interface{}, len(rows))
			for i := range rows {
				out[i] = rows[i]
			}
			payload := p.baseStatePayload(map[string]interface{}{
				"debian_systemd_services": out,
			})
			p.sendWithSpool(ctx, payload)
		}
	}
}

func (p *Probe) systemEntities(snap *collect.SystemSnapshot) map[string]interface{} {
	hostname := p.cfg.Raw.Probe.Hostname
	entities := map[string]interface{}{
		"debian_system_metrics": []interface{}{snap},
	}
	fsRows := collect.FilesystemEntities(snap, hostname)
	if len(fsRows) > 0 {
		out := make([]interface{}, len(fsRows))
		for i := range fsRows {
			out[i] = fsRows[i]
		}
		entities["debian_filesystem"] = out
	}
	return entities
}

func (p *Probe) baseStatePayload(entities map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"state": map[string]interface{}{
			"probe":     p.cfg.Raw.Probe.Name,
			"version":   p.cfg.Raw.Probe.Version,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"identity": map[string]interface{}{
				"host_id":  p.cfg.Raw.Probe.HostID,
				"hostname": p.cfg.Raw.Probe.Hostname,
			},
			"data": map[string]interface{}{
				"entities": entities,
			},
		},
	}
}

func (p *Probe) sendWithSpool(ctx context.Context, payload map[string]interface{}) {
	queued, _ := p.spool.LoadAll()
	for _, item := range queued {
		if err := p.client.PushState(ctx, item); err != nil {
			log.Printf("push queued payload failed: %v", err)
			return
		}
		_ = p.spool.RemoveFirst(1)
	}

	if err := p.client.PushState(ctx, payload); err != nil {
		log.Printf("push payload failed, append to spool: %v", err)
		_ = p.spool.Append(payload)
	}
}
