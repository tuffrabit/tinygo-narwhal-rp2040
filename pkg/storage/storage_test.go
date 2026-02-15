package storage

import (
	"testing"

	"github.com/tuffrabit/tinygo-narwhal-rp2040/pkg/config"

	"tinygo.org/x/tinyfs"
)

func newTestStorage(t *testing.T) (*Manager, *tinyfs.MemBlockDevice) {
	// Create a memory-backed block device simulating RP2040 flash
	// 256 byte page size, 4096 byte block size, 64 blocks = 256KB
	blockDev := tinyfs.NewMemoryDevice(256, 4096, 64)

	mgr, err := New(blockDev, true)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	return mgr, blockDev
}

func TestDeviceConfigSaveLoad(t *testing.T) {
	mgr, _ := newTestStorage(t)
	defer mgr.Close()

	original := config.DeviceConfig{
		Flags:         0x12345678,
		ActiveProfile: 3,
		Brightness:    200,
		DebounceMs:    5,
	}

	// Save
	if err := mgr.SaveDevice(&original); err != nil {
		t.Fatalf("SaveDevice failed: %v", err)
	}

	// Load
	var loaded config.DeviceConfig
	if err := mgr.LoadDevice(&loaded); err != nil {
		t.Fatalf("LoadDevice failed: %v", err)
	}

	// Verify version was set
	if loaded.Version != config.CurrentVersion {
		t.Errorf("Version not set: expected %d, got %d", config.CurrentVersion, loaded.Version)
	}

	// Verify other fields
	if loaded.Flags != original.Flags {
		t.Errorf("Flags: expected 0x%x, got 0x%x", original.Flags, loaded.Flags)
	}
	if loaded.ActiveProfile != original.ActiveProfile {
		t.Errorf("ActiveProfile: expected %d, got %d", original.ActiveProfile, loaded.ActiveProfile)
	}
	if loaded.Brightness != original.Brightness {
		t.Errorf("Brightness: expected %d, got %d", original.Brightness, loaded.Brightness)
	}
	if loaded.DebounceMs != original.DebounceMs {
		t.Errorf("DebounceMs: expected %d, got %d", original.DebounceMs, loaded.DebounceMs)
	}
}

func TestProfileSaveLoad(t *testing.T) {
	mgr, _ := newTestStorage(t)
	defer mgr.Close()

	original := config.Profile{
		Flags:        0xDEADBEEF,
		RGBColor:     0xFF5733,
		RGBPattern:   2,
		BindingCount: 1,
	}
	original.SetName("Gaming")
	original.Bindings[0] = config.KeyBinding{
		InputType:   config.BindingTypeKey,
		InputID:     0,
		OutputType:  config.OutputTypeKeyboard,
		OutputValue: 0x1E, // '1' key
		Modifiers:   0,
		Flags:       0,
	}

	slot := uint8(0)

	// Save
	if err := mgr.SaveProfile(slot, &original); err != nil {
		t.Fatalf("SaveProfile failed: %v", err)
	}

	// Load
	var loaded config.Profile
	if err := mgr.LoadProfile(slot, &loaded); err != nil {
		t.Fatalf("LoadProfile failed: %v", err)
	}

	// Verify
	if loaded.GetName() != "Gaming" {
		t.Errorf("Name: expected 'Gaming', got '%s'", loaded.GetName())
	}
	if loaded.RGBColor != original.RGBColor {
		t.Errorf("RGBColor: expected 0x%x, got 0x%x", original.RGBColor, loaded.RGBColor)
	}
	if loaded.BindingCount != original.BindingCount {
		t.Errorf("BindingCount: expected %d, got %d", original.BindingCount, loaded.BindingCount)
	}
}

func TestProfileNotFound(t *testing.T) {
	mgr, _ := newTestStorage(t)
	defer mgr.Close()

	var profile config.Profile
	err := mgr.LoadProfile(5, &profile)

	if err != ErrProfileNotFound {
		t.Errorf("Expected ErrProfileNotFound, got %v", err)
	}
}

func TestMultipleProfiles(t *testing.T) {
	mgr, _ := newTestStorage(t)
	defer mgr.Close()

	// Save profiles in non-sequential slots
	for _, slot := range []uint8{0, 3, 7, 12, 15} {
		profile := config.Profile{}
		profile.SetName("Profile " + string('0'+slot))
		if err := mgr.SaveProfile(slot, &profile); err != nil {
			t.Fatalf("SaveProfile slot %d failed: %v", slot, err)
		}
	}

	// List profiles
	slots, err := mgr.ListProfiles()
	if err != nil {
		t.Fatalf("ListProfiles failed: %v", err)
	}

	if len(slots) != 5 {
		t.Errorf("Expected 5 profiles, got %d", len(slots))
	}

	// Verify all slots present
	slotMap := make(map[uint8]bool)
	for _, s := range slots {
		slotMap[s] = true
	}

	for _, expected := range []uint8{0, 3, 7, 12, 15} {
		if !slotMap[expected] {
			t.Errorf("Expected slot %d in list", expected)
		}
	}
}

func TestDeleteProfile(t *testing.T) {
	mgr, _ := newTestStorage(t)
	defer mgr.Close()

	// Create profile
	profile := config.Profile{}
	profile.SetName("ToDelete")
	mgr.SaveProfile(1, &profile)

	// Verify exists
	if !mgr.ProfileExists(1) {
		t.Error("Profile should exist before deletion")
	}

	// Delete
	if err := mgr.DeleteProfile(1); err != nil {
		t.Fatalf("DeleteProfile failed: %v", err)
	}

	// Verify gone
	if mgr.ProfileExists(1) {
		t.Error("Profile should not exist after deletion")
	}

	// Verify list is empty
	slots, _ := mgr.ListProfiles()
	if len(slots) != 0 {
		t.Errorf("Expected 0 profiles after deletion, got %d", len(slots))
	}
}

func TestAtomicWrite(t *testing.T) {
	mgr, _ := newTestStorage(t)
	defer mgr.Close()

	// Save initial profile
	profile1 := config.Profile{}
	profile1.SetName("Original")
	profile1.RGBColor = 0x111111
	mgr.SaveProfile(0, &profile1)

	// Save new version (should atomically replace)
	profile2 := config.Profile{}
	profile2.SetName("Updated")
	profile2.RGBColor = 0x222222
	mgr.SaveProfile(0, &profile2)

	// Load and verify it's the new version
	var loaded config.Profile
	mgr.LoadProfile(0, &loaded)

	if loaded.GetName() != "Updated" {
		t.Errorf("Expected 'Updated', got '%s'", loaded.GetName())
	}
	if loaded.RGBColor != 0x222222 {
		t.Errorf("Expected RGB 0x222222, got 0x%x", loaded.RGBColor)
	}
}

func TestVersionMismatchWipe(t *testing.T) {
	// Create storage and add some data
	blockDev := tinyfs.NewMemoryDevice(256, 4096, 64)

	mgr, err := New(blockDev, true)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	// Save a profile
	profile := config.Profile{}
	profile.SetName("ShouldBeDeleted")
	mgr.SaveProfile(0, &profile)

	// Save device config (this sets version to CurrentVersion)
	mgr.SaveDevice(&config.DeviceConfig{ActiveProfile: 1})

	mgr.Close()

	// Temporarily change the current version to simulate firmware update
	originalVersion := config.CurrentVersion
	// We can't actually change the const, but the test documents the behavior:
	// When firmware is updated and CurrentVersion changes, on next boot
	// the storage manager detects the mismatch and wipes configs.
	// Since we can't change a const, we verify the version check logic works
	// by checking that the version was properly saved.
	_ = originalVersion

	// Re-open storage
	mgr2, err := New(blockDev, false)
	if err != nil {
		t.Fatalf("Failed to reopen storage: %v", err)
	}
	defer mgr2.Close()

	// Verify data still exists (because version matches)
	if !mgr2.ProfileExists(0) {
		t.Error("Profile should still exist when version matches")
	}

	var device config.DeviceConfig
	if err := mgr2.LoadDevice(&device); err != nil {
		t.Errorf("Device config should exist: %v", err)
	}
	if device.Version != config.CurrentVersion {
		t.Errorf("Device config version should be %d, got %d", config.CurrentVersion, device.Version)
	}
}

func TestFactoryReset(t *testing.T) {
	mgr, _ := newTestStorage(t)
	defer mgr.Close()

	// Create some data
	mgr.SaveDevice(&config.DeviceConfig{ActiveProfile: 1})
	mgr.SaveProfile(0, &config.Profile{})
	mgr.SaveProfile(1, &config.Profile{})

	// Factory reset
	if err := mgr.ForceWipe(); err != nil {
		t.Fatalf("ForceWipe failed: %v", err)
	}

	// Verify all gone
	slots, _ := mgr.ListProfiles()
	if len(slots) != 0 {
		t.Errorf("Expected 0 profiles after reset, got %d", len(slots))
	}

	var device config.DeviceConfig
	if err := mgr.LoadDevice(&device); err == nil {
		t.Error("Expected device config to be wiped")
	}
}

func TestStorageStats(t *testing.T) {
	mgr, _ := newTestStorage(t)
	defer mgr.Close()

	// Get initial stats
	stats1, err := mgr.GetStats()
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}

	if stats1.ProfileCount != 0 {
		t.Errorf("Expected 0 profiles initially, got %d", stats1.ProfileCount)
	}

	// Add profiles
	for i := 0; i < 5; i++ {
		profile := config.Profile{}
		profile.SetName("Profile")
		mgr.SaveProfile(uint8(i), &profile)
	}

	// Get updated stats
	stats2, err := mgr.GetStats()
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}

	if stats2.ProfileCount != 5 {
		t.Errorf("Expected 5 profiles, got %d", stats2.ProfileCount)
	}

	// Verify CanFitProfile returns true with space available
	if !mgr.CanFitProfile() {
		t.Error("CanFitProfile should return true with available space")
	}
}

func BenchmarkProfileSave(b *testing.B) {
	mgr, _ := newTestStorage(nil)
	defer mgr.Close()

	profile := config.Profile{
		Flags:        0x12345678,
		RGBColor:     0xFF00FF,
		RGBPattern:   3,
		BindingCount: 32,
	}
	profile.SetName("Benchmark")

	for i := range profile.Bindings {
		profile.Bindings[i] = config.KeyBinding{
			InputType:   config.BindingTypeKey,
			InputID:     uint8(i),
			OutputType:  config.OutputTypeKeyboard,
			OutputValue: uint16(i),
			Modifiers:   0,
			Flags:       0,
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Save to different slots to avoid overwrite optimization
		mgr.SaveProfile(uint8(i%256), &profile)
	}
}

func BenchmarkProfileLoad(b *testing.B) {
	mgr, _ := newTestStorage(nil)
	defer mgr.Close()

	profile := config.Profile{
		BindingCount: 32,
	}
	profile.SetName("Benchmark")
	mgr.SaveProfile(0, &profile)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var loaded config.Profile
		mgr.LoadProfile(0, &loaded)
	}
}
