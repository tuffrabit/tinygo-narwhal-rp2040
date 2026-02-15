# Goroutine Architecture for TinyGo USB Gamepad

This document outlines the goroutine architecture recommendations for the RP2040-based USB gamepad, based on analysis of TinyGo 0.40.1 source code.

## Key Findings

### Scheduler: Multicore ("cores" scheduler)

The RP2040 uses the **"cores" scheduler** (not cooperative), which provides true parallelism across both CPU cores:

- Goroutines are automatically distributed across 2 cores
- `hasParallelism = true` - tasks can run simultaneously
- Background tasks on Core 1 won't steal cycles from latency-sensitive code on Core 0

### Default Stack Size

- **Pico target**: 8KB per goroutine (`"default-stack-size": 8192`)
- **Generic Cortex-M fallback**: 2KB per goroutine
- TinyGo has automatic stack size detection enabled, so simple goroutines may use less than the default

### USB Implementation is Non-Blocking

From TinyGo source (`machine/usb/hid/joystick/joystick.go`):

```go
func (m *joystick) tx(b []byte) {
    if m.waitTxc {
        m.buf.Put(b)  // Queue if previous send pending
    } else {
        m.waitTxc = true
        hid.SendUSBPacket(b)  // Hand off to USB hardware
    }
}
```

Key points:
- `SendState()` / `SendReport()` are **non-blocking** - they copy to a ring buffer and return immediately
- Actual USB transmission happens in the interrupt handler (`handleUSBIRQ`) at highest priority (`SetPriority(0x00)`)
- USB host polls at its own rate (~1ms), but you can call `SendState()` faster - the ring buffer absorbs timing differences

## Practical Goroutine Limits

With 256KB RAM on RP2040:

- **Conservative estimate**: ~10-15 goroutines at 8KB each = 80-120KB
- Leaves plenty of room for heap, USB buffers, and application data
- With TinyGo's automatic stack sizing, you may fit more

## Recommended Architecture

```go
func inputLoop() {
    ticker := time.NewTicker(time.Microsecond * 1000) // 1kHz sampling
    defer ticker.Stop()
    
    for range ticker.C {
        // Read GPIO/matrix (tight, fast)
        state := readInputs()
        
        // Update joystick state
        joystick.State.SetButtons(state.buttons)
        joystick.State.SetAxis(joystick.X, state.x)
        // ...
        
        // Queue USB report (non-blocking, returns immediately)
        joystick.SendState()
    }
}

func main() {
    // USB already initialized via init()
    
    // Start high-priority input sampling
    go inputLoop()
    
    // Main handles coordination only
    for {
        select {
        case cmd := <-serialCmdCh:
            handleSerialCommand(cmd)
        case config := <-saveConfigCh:
            saveConfig(config)
        case <-ledUpdateCh:
            updateLEDs()
        }
    }
}
```

## Key Points

1. **`inputLoop` calls `SendState()`** - Yes, it should handle triggering the reports. The USB hardware and interrupt handler manage actual transmission timing.

2. **Main loop is just a coordinator** - It can block on channels waiting for work. USB timing is handled by hardware IRQ, not your main loop.

3. **No busy-looping needed** - Using `time.Ticker` in `inputLoop` gives consistent timing without burning CPU cycles. The RP2040 scheduler yields properly between goroutines.

4. **Ring buffer absorbs timing** - If you sample faster than USB polls (e.g., 2kHz sampling vs 1kHz USB), reports queue up rather than block.

## Optional: Minimal Jitter Optimization

For absolute minimal jitter (competitive gaming scenarios), you could sync to USB SOF (Start of Frame) interrupts. However, a simple 1kHz ticker in `inputLoop` calling `SendState()` is the standard approach and works well for most applications.

## Summary

| Component | Priority | Goroutine | Notes |
|-----------|----------|-----------|-------|
| Input sampling | High | `inputLoop` | Use `time.Ticker`, call `SendState()` |
| Serial commands | Low | Separate | Can block/sleep without impact |
| Config save/load | Low | Separate | Rare operations, channel-triggered |
| LED animations | Low | Separate | `time.Sleep()` between frames |
| Main loop | Coordinator | Main | Blocks on channels, no busy work |
