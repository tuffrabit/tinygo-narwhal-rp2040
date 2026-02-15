// Package storage provides persistent configuration storage using LittleFS.
// It handles atomic writes, version checking, and cleanup of temporary files.
package storage

import (
	"errors"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/tuffrabit/tinygo-narwhal-rp2040/pkg/config"

	"tinygo.org/x/tinyfs"
	"tinygo.org/x/tinyfs/littlefs"
)

const (
	configDir     = "/config"
	profilesDir   = "/config/profiles"
	deviceFile    = "/config/device.bin"
	tempSuffix    = ".tmp"
	profilePrefix = ""
	profileSuffix = ".bin"
)

var (
	ErrProfileNotFound = errors.New("profile not found")
	ErrFlashFull       = errors.New("insufficient flash space")
	ErrInvalidProfile  = errors.New("invalid profile data")
	ErrVersionMismatch = errors.New("config version mismatch")
	ErrFilesystem      = errors.New("filesystem error")
)

// Manager handles config persistence using LittleFS.
type Manager struct {
	fs       *littlefs.LFS
	blockDev tinyfs.BlockDevice
	mounted  bool
}

// Stats provides information about storage usage.
type Stats struct {
	TotalSpace     int64
	UsedSpace      int64
	FreeSpace      int64
	ProfileCount   int
	NeedsMigration bool // true if config version mismatch detected
}

// New initializes the storage system with the given block device.
// It mounts the filesystem and performs boot-time cleanup.
// If format is true and mount fails, it will format the filesystem.
func New(blockDev tinyfs.BlockDevice, format bool) (*Manager, error) {
	lfs := littlefs.New(blockDev)

	// Configure LittleFS for RP2040 flash
	// These are conservative settings for reliability
	lfs.Configure(&littlefs.Config{
		CacheSize:     512,
		LookaheadSize: 128,
	})

	// Try to mount existing filesystem
	err := lfs.Mount()
	if err != nil {
		if !format {
			return nil, err
		}
		// Format and try again
		if err := lfs.Format(); err != nil {
			return nil, err
		}
		if err := lfs.Mount(); err != nil {
			return nil, err
		}
	}

	m := &Manager{
		fs:       lfs,
		blockDev: blockDev,
		mounted:  true,
	}

	// Perform boot-time cleanup
	if err := m.bootCleanup(); err != nil {
		// Log but don't fail - we can still operate
		// TODO: logging
	}

	// Check version and handle migration
	needsWipe, err := m.checkVersion()
	if err != nil {
		// No device config yet or error reading - that's ok for first boot
		needsWipe = false
	}

	if needsWipe {
		// Version mismatch - wipe all configs
		// This is intentional - user must restore from PC app after firmware update
		if err := m.wipeAll(); err != nil {
			return nil, err
		}
	}

	return m, nil
}

// Close unmounts the filesystem.
func (m *Manager) Close() error {
	if m.mounted {
		m.mounted = false
		return m.fs.Unmount()
	}
	return nil
}

// bootCleanup removes temporary files left over from interrupted writes.
func (m *Manager) bootCleanup() error {
	// Clean up temp files in config dir
	entries, err := m.readDir(configDir)
	if err != nil {
		// Config dir might not exist yet
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		name := entry.Name()
		if strings.HasSuffix(name, tempSuffix) {
			tempPath := path.Join(configDir, name)
			m.fs.Remove(tempPath)
		}
	}

	// Clean up temp files in profiles dir
	entries, err = m.readDir(profilesDir)
	if err != nil {
		// Profiles dir might not exist yet
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		name := entry.Name()
		if strings.HasSuffix(name, tempSuffix) {
			tempPath := path.Join(profilesDir, name)
			m.fs.Remove(tempPath)
		}
	}

	return nil
}

// readDir reads the directory entries at the given path.
func (m *Manager) readDir(dirPath string) ([]os.FileInfo, error) {
	f, err := m.fs.Open(dirPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	if !f.IsDir() {
		return nil, errors.New("not a directory")
	}

	return f.Readdir(-1)
}

// checkVersion reads device config and checks if version matches.
// Returns true if configs should be wiped (version mismatch).
func (m *Manager) checkVersion() (bool, error) {
	var deviceCfg config.DeviceConfig
	if err := m.LoadDevice(&deviceCfg); err != nil {
		if os.IsNotExist(err) {
			// No device config yet - not a version mismatch, just first boot
			return false, nil
		}
		return false, err
	}

	return deviceCfg.Version != config.CurrentVersion, nil
}

// wipeAll removes all configuration files.
func (m *Manager) wipeAll() error {
	// Remove all profiles
	slots, err := m.ListProfiles()
	if err == nil {
		for _, slot := range slots {
			m.DeleteProfile(slot)
		}
	}

	// Remove device config
	m.fs.Remove(deviceFile)

	return nil
}

// ensureDirs creates the config directories if they don't exist.
func (m *Manager) ensureDirs() error {
	if err := m.fs.Mkdir(configDir, 0755); err != nil && !isExist(err) {
		return err
	}
	if err := m.fs.Mkdir(profilesDir, 0755); err != nil && !isExist(err) {
		return err
	}
	return nil
}

// isExist checks if an error is "already exists".
// LittleFS errors don't always match os.IsExist, so we check the message too.
func isExist(err error) bool {
	if err == nil {
		return false
	}
	if os.IsExist(err) {
		return true
	}
	// Check for LittleFS specific error message
	return strings.Contains(err.Error(), "already exists")
}

// LoadDevice loads the device configuration.
func (m *Manager) LoadDevice(cfg *config.DeviceConfig) error {
	f, err := m.fs.Open(deviceFile)
	if err != nil {
		return err
	}
	defer f.Close()

	buf := make([]byte, 12)
	n, err := f.Read(buf)
	if err != nil {
		return err
	}
	if n != 12 {
		return ErrInvalidProfile
	}

	return cfg.UnmarshalBinary(buf)
}

// SaveDevice saves the device configuration atomically.
func (m *Manager) SaveDevice(cfg *config.DeviceConfig) error {
	if err := m.ensureDirs(); err != nil {
		return err
	}

	// Set version
	cfg.Version = config.CurrentVersion

	data, err := cfg.MarshalBinary()
	if err != nil {
		return err
	}

	return m.atomicWrite(deviceFile, data)
}

// LoadProfile loads a profile from the given slot.
func (m *Manager) LoadProfile(slot uint8, profile *config.Profile) error {
	profilePath := m.profilePath(slot)

	f, err := m.fs.Open(profilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return ErrProfileNotFound
		}
		// Check if it's a "no directory entry" error from littlefs
		if strings.Contains(err.Error(), "No directory entry") {
			return ErrProfileNotFound
		}
		return err
	}
	defer f.Close()

	buf := make([]byte, 286)
	n, err := f.Read(buf)
	if err != nil {
		return err
	}
	if n != 286 {
		return ErrInvalidProfile
	}

	return profile.UnmarshalBinary(buf)
}

// SaveProfile saves a profile to the given slot atomically.
func (m *Manager) SaveProfile(slot uint8, profile *config.Profile) error {
	if err := m.ensureDirs(); err != nil {
		return err
	}

	// Set version
	profile.Version = config.CurrentVersion

	data, err := profile.MarshalBinary()
	if err != nil {
		return err
	}

	profilePath := m.profilePath(slot)
	return m.atomicWrite(profilePath, data)
}

// DeleteProfile removes a profile from the given slot.
func (m *Manager) DeleteProfile(slot uint8) error {
	profilePath := m.profilePath(slot)
	return m.fs.Remove(profilePath)
}

// ProfileExists checks if a profile exists in the given slot.
func (m *Manager) ProfileExists(slot uint8) bool {
	profilePath := m.profilePath(slot)
	f, err := m.fs.Open(profilePath)
	if err != nil {
		return false
	}
	f.Close()
	return true
}

// ListProfiles returns a list of occupied profile slots.
func (m *Manager) ListProfiles() ([]uint8, error) {
	entries, err := m.readDir(profilesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []uint8{}, nil
		}
		return nil, err
	}

	var slots []uint8
	for _, entry := range entries {
		name := entry.Name()
		// Parse "N.bin" format
		if !strings.HasSuffix(name, profileSuffix) {
			continue
		}
		if strings.HasSuffix(name, tempSuffix) {
			continue // Skip temp files
		}

		numStr := strings.TrimPrefix(name, profilePrefix)
		numStr = strings.TrimSuffix(numStr, profileSuffix)

		if slot, err := strconv.ParseUint(numStr, 10, 8); err == nil {
			slots = append(slots, uint8(slot))
		}
	}

	return slots, nil
}

// GetStats returns storage statistics.
func (m *Manager) GetStats() (*Stats, error) {
	// LittleFS doesn't have a direct "free space" call
	// We can estimate by trying to allocate or tracking usage

	profiles, err := m.ListProfiles()
	if err != nil {
		// Directory might not exist yet (no profiles saved)
		if strings.Contains(err.Error(), "No directory entry") {
			profiles = []uint8{}
		} else {
			return nil, err
		}
	}

	// Estimate space used
	// Each profile: 286 bytes data + ~32 bytes LittleFS overhead = ~320 bytes
	// Device config: 12 bytes + ~32 bytes overhead = ~44 bytes
	// Plus directory entries
	used := int64(len(profiles)*320 + 100)

	// Total space is from the block device
	total := m.blockDev.Size()

	return &Stats{
		TotalSpace:   total,
		UsedSpace:    used,
		FreeSpace:    total - used,
		ProfileCount: len(profiles),
	}, nil
}

// CanFitProfile estimates if a new profile can be stored.
// This is a conservative estimate.
func (m *Manager) CanFitProfile() bool {
	stats, err := m.GetStats()
	if err != nil {
		return false
	}
	// Need space for profile + overhead + potential block alignment
	return stats.FreeSpace > 512
}

// profilePath returns the filesystem path for a profile slot.
func (m *Manager) profilePath(slot uint8) string {
	return path.Join(profilesDir, strconv.Itoa(int(slot))+profileSuffix)
}

// atomicWrite writes data to a temporary file, syncs it, then renames.
// This ensures atomic updates - the original file is never in a partially written state.
func (m *Manager) atomicWrite(filepath string, data []byte) error {
	tempPath := filepath + tempSuffix

	// Remove temp file if it exists (from interrupted previous write)
	m.fs.Remove(tempPath)

	// Write to temp file
	f, err := m.fs.OpenFile(tempPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
	if err != nil {
		return err
	}

	if _, err := f.Write(data); err != nil {
		f.Close()
		m.fs.Remove(tempPath)
		return err
	}

	// CRITICAL: Sync ensures data hits flash
	// Type assert to *littlefs.File to access Sync()
	if syncer, ok := f.(interface{ Sync() error }); ok {
		if err := syncer.Sync(); err != nil {
			f.Close()
			m.fs.Remove(tempPath)
			return err
		}
	}

	if err := f.Close(); err != nil {
		m.fs.Remove(tempPath)
		return err
	}

	// Remove existing file if present (LittleFS rename doesn't replace)
	m.fs.Remove(filepath)

	// Atomic rename
	if err := m.fs.Rename(tempPath, filepath); err != nil {
		m.fs.Remove(tempPath)
		return err
	}

	return nil
}

// ForceWipe completely erases all configuration (for testing/debugging).
func (m *Manager) ForceWipe() error {
	return m.wipeAll()
}
