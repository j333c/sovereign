package supervisor

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
	"sovereign/internal/store"
)

// RevisionEngine defines the declarative Compose revisioning and pre-upgrade snapshot operations.
type RevisionEngine interface {
	CreatePreUpgradeRevision(ctx context.Context, serviceID string, newImageTag string) (*store.ComposeRevision, error)
	UpgradeService(ctx context.Context, serviceID string, newImageTag string) (*store.ComposeRevision, error)
}

// SynthesizeRevisionCompose writes an atomic, declarative Compose v2 file for the specified revision at:
// /data/state/compose/rev-:rev.yaml.
func SynthesizeRevisionCompose(dataDir string, revision int, spec ContainerSpec) (string, error) {
	composeDir := filepath.Join(dataDir, "state", "compose")
	if err := os.MkdirAll(composeDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create compose directory: %w", err)
	}

	ports := make([]string, 0, len(spec.Ports)+len(spec.UDPPorts))
	tcpKeys := make([]int, 0, len(spec.Ports))
	for k := range spec.Ports {
		tcpKeys = append(tcpKeys, k)
	}
	sort.Ints(tcpKeys)
	for _, hostPort := range tcpKeys {
		ports = append(ports, fmt.Sprintf("127.0.0.1:%d:%d", hostPort, spec.Ports[hostPort]))
	}

	udpKeys := make([]int, 0, len(spec.UDPPorts))
	for k := range spec.UDPPorts {
		udpKeys = append(udpKeys, k)
	}
	sort.Ints(udpKeys)
	for _, hostPort := range udpKeys {
		ports = append(ports, fmt.Sprintf("%d:%d/udp", hostPort, spec.UDPPorts[hostPort]))
	}

	limitMB := spec.MemoryLimitBytes / (1024 * 1024)
	resMB := spec.MemoryReservationBytes / (1024 * 1024)

	networks := spec.Networks
	if len(networks) == 0 {
		networks = []string{"sovereign-ingress", "sovereign-control"}
	}
	networksMap := make(map[string]map[string]any)
	for _, n := range networks {
		networksMap[n] = map[string]any{"external": true}
	}

	var deploy *ComposeDeploy
	if limitMB > 0 || resMB > 0 {
		deploy = &ComposeDeploy{
			Resources: ComposeResources{
				Limits: ComposeResourceLimits{
					Memory: fmt.Sprintf("%dM", limitMB),
				},
				Reservations: ComposeResourceLimits{
					Memory: fmt.Sprintf("%dM", resMB),
				},
			},
		}
	}

	composeData := ComposeFile{
		Services: map[string]ComposeServiceSpec{
			spec.Name: {
				Image:         spec.Image,
				ContainerName: "sovereign-" + spec.Name,
				Restart:       "unless-stopped",
				Ports:         ports,
				Volumes:       spec.Volumes,
				Devices:       spec.Devices,
				Environment:   spec.Env,
				Command:       spec.Command,
				Deploy:        deploy,
				Networks:      networks,
			},
		},
		Networks: networksMap,
	}

	yamlBytes, err := yaml.Marshal(composeData)
	if err != nil {
		return "", fmt.Errorf("failed to marshal compose yaml: %w", err)
	}

	destPath := filepath.Join(composeDir, fmt.Sprintf("rev-%d.yaml", revision))
	tmpPath := destPath + fmt.Sprintf(".tmp-%d", time.Now().UnixNano())

	f, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return "", fmt.Errorf("failed to create temp revision compose file: %w", err)
	}

	if _, err := f.Write(yamlBytes); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("failed to write temp revision compose file: %w", err)
	}

	if err := f.Sync(); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("failed to fsync temp revision compose file: %w", err)
	}

	if err := f.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return "", err
	}

	if err := os.Rename(tmpPath, destPath); err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("failed to atomically rename revision compose file: %w", err)
	}

	return destPath, nil
}

// CopyDir recursively copies directory src into dst, creating directories as needed.
func CopyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to stat source directory %s: %w", src, err)
	}

	if !srcInfo.IsDir() {
		return copyFile(src, dst)
	}

	if err := os.MkdirAll(dst, srcInfo.Mode()|0700); err != nil {
		return fmt.Errorf("failed to create target directory %s: %w", dst, err)
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("failed to read source directory %s: %w", src, err)
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.Type()&os.ModeSymlink != 0 {
			linkTarget, err := os.Readlink(srcPath)
			if err == nil {
				_ = os.Symlink(linkTarget, dstPath)
			}
			continue
		}

		if entry.IsDir() {
			if err := CopyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("failed to stat source file %s: %w", src, err)
	}

	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file %s: %w", src, err)
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return fmt.Errorf("failed to create destination dir for file %s: %w", dst, err)
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return fmt.Errorf("failed to create destination file %s: %w", dst, err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("failed to copy file contents from %s to %s: %w", src, dst, err)
	}
	return out.Sync()
}

// SnapshotConfigurationVolumes creates a directory backup of the service's configuration volumes
// at /data/state/backups/volumes-rev-:rev/<service>.
func SnapshotConfigurationVolumes(dataDir string, revision int, serviceID string, volumeTemplates []string) (string, error) {
	backupDir := filepath.Join(dataDir, "state", "backups", fmt.Sprintf("volumes-rev-%d", revision), serviceID)
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create volume backup directory: %w", err)
	}

	// 1. Service state directory under /data/state/services/<serviceID>
	serviceDir := filepath.Join(dataDir, "state", "services", serviceID)
	if fi, err := os.Stat(serviceDir); err == nil && fi.IsDir() {
		if err := CopyDir(serviceDir, backupDir); err != nil {
			return "", fmt.Errorf("failed to copy service state directory: %w", err)
		}
	}

	// 2. Direct state directory under /data/state/<serviceID>
	altDir := filepath.Join(dataDir, "state", serviceID)
	if altDir != serviceDir {
		if fi, err := os.Stat(altDir); err == nil && fi.IsDir() {
			if err := CopyDir(altDir, backupDir); err != nil {
				return "", fmt.Errorf("failed to copy alt state directory: %w", err)
			}
		}
	}

	// 3. For services like paperless where configuration or metadata is under documents/data
	if serviceID == "paperless" {
		paperlessDataDir := filepath.Join(dataDir, "documents", "data")
		if fi, err := os.Stat(paperlessDataDir); err == nil && fi.IsDir() {
			_ = CopyDir(paperlessDataDir, filepath.Join(backupDir, "data"))
		}
	}

	// 4. Any volume explicitly pointing to a state directory
	for _, t := range volumeTemplates {
		parts := strings.Split(t, ":")
		hostRel := parts[0]
		if strings.HasPrefix(hostRel, "state/") {
			hostAbs := filepath.Join(dataDir, hostRel)
			if hostAbs == serviceDir || hostAbs == altDir {
				continue
			}
			if fi, err := os.Stat(hostAbs); err == nil {
				subDst := filepath.Join(backupDir, filepath.Base(hostAbs))
				if fi.IsDir() {
					_ = CopyDir(hostAbs, subDst)
				} else {
					_ = copyFile(hostAbs, subDst)
				}
			}
		}
	}

	return backupDir, nil
}

// buildContainerSpec constructs a ContainerSpec for a service with the specified image.
func (m *ServiceManager) buildContainerSpec(ctx context.Context, id string, image string) (ContainerSpec, error) {
	id = m.resolveID(id)
	m.mu.RLock()
	def, exists := m.definitions[id]
	m.mu.RUnlock()

	if !exists {
		return ContainerSpec{}, ErrServiceNotFound
	}

	if image == "" {
		image = def.Image
	}

	var devices []string
	var udpPorts map[int]int
	var env map[string]string

	if id == ServiceJellyfin && m.jellyfinProvisioner != nil {
		if res, err := m.jellyfinProvisioner.Provision(ctx); err == nil && res != nil {
			devices = res.Devices
			udpPorts = res.UDPPorts
		}
	} else if (id == ServiceNavidrome || id == ServiceAudiobookshelf) && m.audioProvisioner != nil {
		if res, err := m.audioProvisioner.Provision(ctx, id); err == nil && res != nil {
			env = res.Env
			devices = res.Devices
		}
	} else if id == ServicePaperless && m.paperlessProvisioner != nil {
		if res, err := m.paperlessProvisioner.Provision(ctx); err == nil && res != nil {
			env = res.Env
			devices = res.Devices
		}
	}

	if id == ServiceLlama && m.llamaProvisioner != nil {
		if res, err := m.llamaProvisioner.Provision(ctx); err == nil && res != nil && !res.InSafeStub {
			spec := res.ContainerSpec
			spec.Image = image
			return spec, nil
		}
	} else if id == ServiceOpenWebUI && m.openwebuiProvisioner != nil {
		if res, err := m.openwebuiProvisioner.Provision(ctx); err == nil && res != nil {
			spec := res.ContainerSpec
			spec.Image = image
			return spec, nil
		}
	}

	volumes := resolveVolumes(m.dataDir, def.VolumeTemplates)
	return ContainerSpec{
		Name:                   def.ID,
		Image:                  image,
		MemoryLimitBytes:       def.MemLimitBytes,
		MemoryReservationBytes: def.MemReservationBytes,
		Ports:                  map[int]int{def.Port: def.Port},
		UDPPorts:               udpPorts,
		Devices:                devices,
		Volumes:                volumes,
		Networks:               []string{"sovereign-ingress", "sovereign-control"},
		Env:                    env,
	}, nil
}

// CreatePreUpgradeRevision generates declarative compose rev file, atomic SQLite snapshot,
// and volume backup before applying updates.
func (m *ServiceManager) CreatePreUpgradeRevision(ctx context.Context, serviceID string, newImageTag string) (*store.ComposeRevision, error) {
	id := m.resolveID(serviceID)

	m.mu.RLock()
	def, exists := m.definitions[id]
	st, stExists := m.statuses[id]
	m.mu.RUnlock()

	if !exists {
		return nil, ErrServiceNotFound
	}

	oldImage := def.Image
	if stExists && st.Image != "" {
		oldImage = st.Image
	}

	// Resolve new image
	newImage := oldImage
	if newImageTag != "" && newImageTag != oldImage {
		if strings.Contains(newImageTag, "/") {
			newImage = newImageTag
		} else {
			lastColon := strings.LastIndex(oldImage, ":")
			if lastColon != -1 {
				repo := oldImage[:lastColon]
				tag := strings.TrimPrefix(newImageTag, ":")
				newImage = fmt.Sprintf("%s:%s", repo, tag)
			} else {
				tag := strings.TrimPrefix(newImageTag, ":")
				newImage = fmt.Sprintf("%s:%s", oldImage, tag)
			}
		}
	}

	// 1. Allocate next revision number atomically
	revNum := 1
	if m.revisionStore != nil {
		var err error
		revNum, err = m.revisionStore.GetNextRevisionNumber(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to allocate next revision number: %w", err)
		}
	}

	// 2. Create atomic SQLite snapshot at /data/state/backups/pre-upgrade-:rev.db
	sqliteSnapshotPath := filepath.Join(m.dataDir, "state", "backups", fmt.Sprintf("pre-upgrade-%d.db", revNum))
	if m.revisionStore != nil {
		if err := m.revisionStore.CreateSnapshot(ctx, sqliteSnapshotPath); err != nil {
			return nil, fmt.Errorf("failed to create pre-upgrade SQLite snapshot: %w", err)
		}
	}

	// 3. Snapshot service configuration volumes to /data/state/backups/volumes-rev-:rev/<service>
	volumeBackupPath, err := SnapshotConfigurationVolumes(m.dataDir, revNum, id, def.VolumeTemplates)
	if err != nil {
		return nil, fmt.Errorf("failed to snapshot configuration volumes: %w", err)
	}

	// 4. Generate declarative versioned Compose specification at /data/state/compose/rev-:rev.yaml
	containerSpec, err := m.buildContainerSpec(ctx, id, newImage)
	if err != nil {
		return nil, fmt.Errorf("failed to construct container specification: %w", err)
	}

	composeFilePath, err := SynthesizeRevisionCompose(m.dataDir, revNum, containerSpec)
	if err != nil {
		return nil, fmt.Errorf("failed to synthesize revision compose file: %w", err)
	}

	// 5. Persist revision record in compose_revisions and mark it as active revision
	rev := &store.ComposeRevision{
		Revision:           revNum,
		ServiceID:          id,
		OldImage:           oldImage,
		NewImage:           newImage,
		ComposeFilePath:    composeFilePath,
		SQLiteSnapshotPath: sqliteSnapshotPath,
		VolumeBackupPath:   volumeBackupPath,
		Status:             "active",
	}

	if m.revisionStore != nil {
		if err := m.revisionStore.RecordRevision(ctx, rev); err != nil {
			return nil, fmt.Errorf("failed to record revision in store: %w", err)
		}
	}

	m.logger.Info("created pre-upgrade revision",
		"revision", revNum,
		"service", id,
		"old_image", oldImage,
		"new_image", newImage,
		"compose_path", composeFilePath,
		"snapshot_path", sqliteSnapshotPath,
		"volume_path", volumeBackupPath,
	)

	return rev, nil
}

// UpgradeService performs a complete service upgrade with pre-upgrade snapshot and revision tracking.
func (m *ServiceManager) UpgradeService(ctx context.Context, serviceID string, newImageTag string) (*store.ComposeRevision, error) {
	id := m.resolveID(serviceID)

	m.mu.RLock()
	_, exists := m.definitions[id]
	st, stExists := m.statuses[id]
	m.mu.RUnlock()

	if !exists {
		return nil, ErrServiceNotFound
	}

	// 1. Create Pre-Upgrade Revision (snapshot DB, backup volumes, write rev-:rev.yaml, record revision)
	rev, err := m.CreatePreUpgradeRevision(ctx, serviceID, newImageTag)
	if err != nil {
		return nil, err
	}

	// 2. Pull new container image if not present
	if m.engine != nil && rev.NewImage != "" {
		present, _ := m.engine.IsImagePresent(ctx, rev.NewImage)
		if !present {
			m.logger.Info("pulling upgraded service image", "service", id, "image", rev.NewImage)
			progressCb := func(percent float64, status string) {
				if m.broadcaster != nil {
					m.broadcaster("service.pull_progress", ServicePullProgress{
						ServiceID:       id,
						ProgressPercent: percent,
						Status:          status,
					})
				}
			}
			if err := m.engine.PullImage(ctx, rev.NewImage, progressCb); err != nil {
				m.logger.Error("failed to pull image for upgrade", "service", id, "error", err)
				if m.revisionStore != nil {
					_ = m.revisionStore.UpdateRevisionStatus(ctx, rev.Revision, "failed")
				}
				return nil, fmt.Errorf("%w: %v", ErrPullFailed, err)
			}
		}
	}

	// 3. Update canonical compose file and runtime container
	containerSpec, err := m.buildContainerSpec(ctx, id, rev.NewImage)
	if err == nil {
		_, _ = SynthesizeComposeSpec(m.dataDir, containerSpec)
	}

	isRunning := stExists && (st.Status == StatusRunning || st.Status == StatusStarting)
	if isRunning && m.engine != nil && err == nil {
		_ = m.engine.StopContainer(ctx, id)
		if err := m.engine.StartContainer(ctx, containerSpec); err != nil {
			m.logger.Error("failed to start upgraded container", "service", id, "error", err)
			if m.revisionStore != nil {
				_ = m.revisionStore.UpdateRevisionStatus(ctx, rev.Revision, "failed")
			}
			return nil, fmt.Errorf("failed to start upgraded container: %w", err)
		}
	}

	// 4. Update status record
	m.mu.Lock()
	if st, exists := m.statuses[id]; exists {
		st.Image = rev.NewImage
		st.ImageDownloaded = true
		st.UpdatedAt = m.timeNow().UTC().Format(time.RFC3339)
	}
	// Also update definitions so subsequent restarts use the upgraded image
	if def, exists := m.definitions[id]; exists {
		def.Image = rev.NewImage
		m.definitions[id] = def
	}
	m.mu.Unlock()

	return rev, nil
}
