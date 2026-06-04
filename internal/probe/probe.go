package probe

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"fluid/probes/core"
	"fluid/probes/core/controlplane"
	"fluid/probes/debian/internal/collect"
	"fluid/probes/debian/internal/config"
	"fluid/probes/debian/internal/shipper"
)

var errInvalidProbeSetup = errors.New("probe: invalid constructor arguments")

// Probe runs Debian host collection loops with optional control plane runtime_config overlay.
type Probe struct {
	rootCtx context.Context
	baseCfg *config.Config
	cfgMu   sync.RWMutex
	cfg     *config.Runtime

	cpClient    *controlplane.HTTPClient
	runtimeSync *controlplane.RuntimeSync
	shipper     *shipper.Client
	spool       *shipper.Spool

	loopMu     sync.Mutex
	loopCancel context.CancelFunc
	wg         sync.WaitGroup
}

// New builds a probe from local config and a control plane HTTP client.
func New(base *config.Runtime, cpClient *controlplane.HTTPClient, sync *controlplane.RuntimeSync) (*Probe, error) {
	if base == nil || cpClient == nil || sync == nil {
		return nil, errInvalidProbeSetup
	}
	baseCopy := base.Raw
	return &Probe{
		baseCfg:     &baseCopy,
		cfg:         base,
		cpClient:    cpClient,
		runtimeSync: sync,
		shipper: shipper.New(
			base.Raw.Controlplane.BaseURL,
			base.Raw.Auth.OrganizationUUID,
			base.Raw.Auth.Token,
			base.RequestTimeout,
		),
		spool: shipper.NewSpool(base.Raw.Spool.Path, base.Raw.Spool.MaxEntries),
	}, nil
}

// ApplyRuntimeJSON parses and merges control plane runtime_config, then reloads collection loops.
func (p *Probe) ApplyRuntimeJSON(raw []byte, configVersion string) error {
	overlay, version, err := config.ParseRuntimeResponse(raw)
	if err != nil {
		return err
	}
	if version == "" {
		version = configVersion
	}
	merged, err := config.ApplyRuntimeOverlay(p.baseCfg, overlay)
	if err != nil {
		return err
	}
	rt, err := config.NewRuntime(merged)
	if err != nil {
		return err
	}
	p.reload(rt, version)
	log.Printf("runtime config applied from control plane (version=%q, files=%d, directories=%d)",
		version, len(rt.Raw.Files), len(rt.Raw.Directories))
	return nil
}

func (p *Probe) reload(rt *config.Runtime, configVersion string) {
	p.cfgMu.Lock()
	p.cfg = rt
	p.cfgMu.Unlock()
	if configVersion != "" {
		p.runtimeSync.SetVersion(configVersion)
	}

	p.loopMu.Lock()
	wasRunning := p.loopCancel != nil
	if p.loopCancel != nil {
		p.loopCancel()
	}
	p.loopMu.Unlock()
	if wasRunning {
		p.wg.Wait()
		if p.rootCtx != nil && p.rootCtx.Err() == nil {
			p.startLoops(p.rootCtx)
		}
	}
}

func (p *Probe) getRuntime() *config.Runtime {
	p.cfgMu.RLock()
	defer p.cfgMu.RUnlock()
	return p.cfg
}

// Start registers with the control plane, pushes schema when present, fetches runtime config, and starts loops.
func (p *Probe) Start(ctx context.Context, schemaPath string) error {
	p.rootCtx = ctx
	name := p.getRuntime().Raw.Probe.Name
	version := p.getRuntime().Raw.Probe.Version
	if err := p.cpClient.Register(name, version); err != nil {
		log.Printf("register warning: %v", err)
	} else if schemaPath != "" {
		if schema, err := core.LoadSchema(schemaPath); err != nil {
			log.Printf("schema load warning: %v", err)
		} else if err := p.cpClient.PushSchema(schema.ToMap()); err != nil {
			log.Printf("schema push warning: %v", err)
		}
	}

	apply := func(raw []byte, ver string) error {
		return p.ApplyRuntimeJSON(raw, ver)
	}
	if err := p.runtimeSync.FetchAndApply(apply); err != nil {
		log.Printf("fetch runtime config at startup failed (using local config only): %v", err)
	}

	p.startLoops(ctx)

	heartbeatEvery := p.getRuntime().SystemInterval
	if heartbeatEvery <= 0 {
		heartbeatEvery = 30 * time.Second
	}
	go p.runtimeSync.RunHeartbeat(ctx, heartbeatEvery, apply)

	<-ctx.Done()
	p.loopMu.Lock()
	if p.loopCancel != nil {
		p.loopCancel()
	}
	p.loopMu.Unlock()
	p.wg.Wait()
	return nil
}

func (p *Probe) startLoops(ctx context.Context) {
	loopCtx, cancel := context.WithCancel(ctx)
	p.loopMu.Lock()
	p.loopCancel = cancel
	p.loopMu.Unlock()

	p.wg.Add(5)
	go p.runSystemLoop(loopCtx)
	go p.runFilesLoop(loopCtx)
	go p.runAptLoop(loopCtx)
	go p.runInstalledPackagesLoop(loopCtx)
	go p.runServicesLoop(loopCtx)
}

func (p *Probe) Stop(_ context.Context) error {
	p.loopMu.Lock()
	if p.loopCancel != nil {
		p.loopCancel()
	}
	p.loopMu.Unlock()
	p.wg.Wait()
	return nil
}

func (p *Probe) runSystemLoop(ctx context.Context) {
	defer p.wg.Done()
	for {
		rt := p.getRuntime()
		if rt == nil {
			return
		}
		timer := time.NewTimer(rt.SystemInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			snap, err := collect.CollectSystem(ctx)
			if err != nil {
				log.Printf("system collect failed: %v", err)
				continue
			}
			payload := p.baseStatePayload(rt, p.systemEntities(rt, snap))
			p.sendWithSpool(ctx, payload)
		}
	}
}

func (p *Probe) runFilesLoop(ctx context.Context) {
	defer p.wg.Done()
	for {
		rt := p.getRuntime()
		if rt == nil {
			return
		}
		timer := time.NewTimer(rt.FilesInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			rows := make([]interface{}, 0, len(rt.Raw.Files)+len(rt.Raw.Directories))
			for _, rule := range rt.Raw.Files {
				rows = append(rows, collect.CollectFile(rule.Path, rt.Raw.Collection.MaxHashFileSize))
			}
			for _, rule := range rt.Raw.Directories {
				rows = append(rows, collect.CollectDirectory(rule.Path, rule.Recursive, rt.Raw.Collection.MaxHashFileSize))
			}
			payload := p.baseStatePayload(rt, map[string]interface{}{"debian_file_checks": rows})
			p.sendWithSpool(ctx, payload)
		}
	}
}

func (p *Probe) runAptLoop(ctx context.Context) {
	defer p.wg.Done()
	for {
		rt := p.getRuntime()
		if rt == nil {
			return
		}
		timer := time.NewTimer(rt.APTInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			snap := collect.CollectAPT(ctx, rt.Raw.Collection.MaxPackageItems)
			payload := p.baseStatePayload(rt, map[string]interface{}{
				"debian_package_updates": []interface{}{snap},
			})
			p.sendWithSpool(ctx, payload)
		}
	}
}

func (p *Probe) runInstalledPackagesLoop(ctx context.Context) {
	defer p.wg.Done()
	p.pushInstalledPackagesSnapshot(ctx)
	for {
		rt := p.getRuntime()
		if rt == nil {
			return
		}
		timer := time.NewTimer(rt.InstalledPackagesInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			p.pushInstalledPackagesSnapshot(ctx)
		}
	}
}

func (p *Probe) pushInstalledPackagesSnapshot(ctx context.Context) {
	rt := p.getRuntime()
	if rt == nil {
		return
	}
	rows, err := collect.CollectInstalledPackages(ctx)
	if err != nil {
		log.Printf("installed packages collect failed: %v", err)
		return
	}
	out := make([]interface{}, len(rows))
	for i := range rows {
		out[i] = rows[i]
	}
	payload := p.baseStatePayload(rt, map[string]interface{}{"debian_installed_packages": out})
	p.sendWithSpool(ctx, payload)
}

func (p *Probe) runServicesLoop(ctx context.Context) {
	defer p.wg.Done()
	for {
		rt := p.getRuntime()
		if rt == nil {
			return
		}
		timer := time.NewTimer(rt.ServicesInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			rows, err := collect.CollectEnabledServices(ctx)
			if err != nil {
				log.Printf("enabled services collect failed: %v", err)
				continue
			}
			out := make([]interface{}, len(rows))
			for i := range rows {
				out[i] = rows[i]
			}
			payload := p.baseStatePayload(rt, map[string]interface{}{"debian_systemd_services": out})
			p.sendWithSpool(ctx, payload)
		}
	}
}

func (p *Probe) systemEntities(rt *config.Runtime, snap *collect.SystemSnapshot) map[string]interface{} {
	hostname := rt.Raw.Probe.Hostname
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

func (p *Probe) baseStatePayload(rt *config.Runtime, entities map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"state": map[string]interface{}{
			"probe":     rt.Raw.Probe.Name,
			"version":   rt.Raw.Probe.Version,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"identity": map[string]interface{}{
				"host_id":  rt.Raw.Probe.HostID,
				"hostname": rt.Raw.Probe.Hostname,
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
		if err := p.shipper.PushState(ctx, item); err != nil {
			log.Printf("push queued payload failed: %v", err)
			return
		}
		_ = p.spool.RemoveFirst(1)
	}
	if err := p.shipper.PushState(ctx, payload); err != nil {
		log.Printf("push payload failed, append to spool: %v", err)
		_ = p.spool.Append(payload)
	}
}
