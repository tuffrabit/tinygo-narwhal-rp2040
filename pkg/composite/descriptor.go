// Package composite provides a custom USB composite device descriptor
// that combines CDC (Serial) + HID (Keyboard + Mouse + Consumer + Gamepad)
package composite

import (
	"machine/usb"
	"machine/usb/descriptor"
)

// CompositeHIDReportDescriptor combines all HID device reports using Report IDs
// This descriptor is the key to making composite HID work with TinyGo
var CompositeHIDReportDescriptor = descriptor.Append([][]byte{
	// ===================================================================
	// REPORT ID 1: MOUSE (5 bytes total: 1 ID + 1 buttons + 3 axes)
	// ===================================================================
	descriptor.HIDUsagePageGenericDesktop,
	descriptor.HIDUsageDesktopMouse,
	descriptor.HIDCollectionApplication,
	descriptor.HIDUsageDesktopPointer,
	descriptor.HIDCollectionPhysical,
	descriptor.HIDReportID(1),
	// Buttons (5 buttons, 1 bit each + 3 bits padding)
	descriptor.HIDUsagePageButton,
	descriptor.HIDUsageMinimum(1),
	descriptor.HIDUsageMaximum(5),
	descriptor.HIDLogicalMinimum(0),
	descriptor.HIDLogicalMaximum(1),
	descriptor.HIDReportCount(5),
	descriptor.HIDReportSize(1),
	descriptor.HIDInputDataVarAbs,
	descriptor.HIDReportCount(1),
	descriptor.HIDReportSize(3),
	descriptor.HIDInputConstVarAbs,
	// Axes (X, Y, Wheel)
	descriptor.HIDUsagePageGenericDesktop,
	descriptor.HIDUsageDesktopX,
	descriptor.HIDUsageDesktopY,
	descriptor.HIDUsageDesktopWheel,
	descriptor.HIDLogicalMinimum(-127),
	descriptor.HIDLogicalMaximum(127),
	descriptor.HIDReportSize(8),
	descriptor.HIDReportCount(3),
	descriptor.HIDInputDataVarRel,
	descriptor.HIDCollectionEnd,
	descriptor.HIDCollectionEnd,

	// ===================================================================
	// REPORT ID 2: KEYBOARD (9 bytes total: 1 ID + 8 data)
	// ===================================================================
	descriptor.HIDUsagePageGenericDesktop,
	descriptor.HIDUsageDesktopKeyboard,
	descriptor.HIDCollectionApplication,
	descriptor.HIDReportID(2),
	// Modifier keys (8 bits)
	descriptor.HIDUsagePageKeyboard,
	descriptor.HIDUsageMinimum(224),
	descriptor.HIDUsageMaximum(231),
	descriptor.HIDLogicalMinimum(0),
	descriptor.HIDLogicalMaximum(1),
	descriptor.HIDReportSize(1),
	descriptor.HIDReportCount(8),
	descriptor.HIDInputDataVarAbs,
	// Reserved byte
	descriptor.HIDReportCount(1),
	descriptor.HIDReportSize(8),
	descriptor.HIDInputConstVarAbs,
	// LED output report (for keyboard LEDs)
	descriptor.HIDReportCount(3),
	descriptor.HIDReportSize(1),
	descriptor.HIDUsagePageLED,
	descriptor.HIDUsageMinimum(1),
	descriptor.HIDUsageMaximum(3),
	descriptor.HIDOutputDataVarAbs,
	descriptor.HIDReportCount(5),
	descriptor.HIDReportSize(1),
	descriptor.HIDOutputConstVarAbs,
	// Keycodes (6 keys)
	descriptor.HIDReportCount(6),
	descriptor.HIDReportSize(8),
	descriptor.HIDLogicalMinimum(0),
	descriptor.HIDLogicalMaximum(255),
	descriptor.HIDUsagePageKeyboard,
	descriptor.HIDUsageMinimum(0),
	descriptor.HIDUsageMaximum(255),
	descriptor.HIDInputDataAryAbs,
	descriptor.HIDCollectionEnd,

	// ===================================================================
	// REPORT ID 3: CONSUMER CONTROL (3 bytes total: 1 ID + 2 data)
	// ===================================================================
	descriptor.HIDUsagePageConsumer,
	descriptor.HIDUsageConsumerControl,
	descriptor.HIDCollectionApplication,
	descriptor.HIDReportID(3),
	descriptor.HIDLogicalMinimum(0),
	descriptor.HIDLogicalMaximum(8191),
	descriptor.HIDUsageMinimum(0),
	descriptor.HIDUsageMaximum(0x1FFF),
	descriptor.HIDReportSize(16),
	descriptor.HIDReportCount(1),
	descriptor.HIDInputDataAryAbs,
	descriptor.HIDCollectionEnd,

	// ===================================================================
	// REPORT ID 4: GAMEPAD (7 bytes total: 1 ID + 2 buttons + 4 axes)
	// Based on Adafruit CircuitPython gamepad descriptor
	// ===================================================================
	descriptor.HIDUsagePageGenericDesktop,
	descriptor.HIDUsageDesktopGamepad,
	descriptor.HIDCollectionApplication,
	descriptor.HIDReportID(4),
	// 16 Buttons (2 bytes)
	descriptor.HIDUsagePageButton,
	descriptor.HIDUsageMinimum(1),
	descriptor.HIDUsageMaximum(16),
	descriptor.HIDLogicalMinimum(0),
	descriptor.HIDLogicalMaximum(1),
	descriptor.HIDReportSize(1),
	descriptor.HIDReportCount(16),
	descriptor.HIDInputDataVarAbs,
	// 4 Analog Axes: X, Y, Z, Rz (4 bytes)
	descriptor.HIDUsagePageGenericDesktop,
	descriptor.HIDLogicalMinimum(-127),
	descriptor.HIDLogicalMaximum(127),
	descriptor.HIDUsageDesktopX,
	descriptor.HIDUsageDesktopY,
	descriptor.HIDUsageDesktopZ,
	descriptor.HIDUsageDesktopRz,
	descriptor.HIDReportSize(8),
	descriptor.HIDReportCount(4),
	descriptor.HIDInputDataVarAbs,
	descriptor.HIDCollectionEnd,
})

// USBDescriptor is the complete USB descriptor for our composite device
// It combines CDC (Serial) + HID (Keyboard/Mouse/Consumer/Gamepad)
var USBDescriptor = descriptor.Descriptor{
	// Device descriptor: USB 2.0 Composite device
	Device: descriptor.DeviceCDC.Bytes(),

	// Configuration descriptor: All interfaces combined
	Configuration: descriptor.Append([][]byte{
		// Configuration header
		descriptor.ConfigurationCDCHID.Bytes(),
		// CDC interfaces
		descriptor.InterfaceAssociationCDC.Bytes(),
		descriptor.InterfaceCDCControl.Bytes(),
		descriptor.ClassSpecificCDCHeader.Bytes(),
		descriptor.ClassSpecificCDCACM.Bytes(),
		descriptor.ClassSpecificCDCUnion.Bytes(),
		descriptor.ClassSpecificCDCCallManagement.Bytes(),
		descriptor.EndpointEP1IN.Bytes(),
		descriptor.InterfaceCDCData.Bytes(),
		descriptor.EndpointEP2OUT.Bytes(),
		descriptor.EndpointEP3IN.Bytes(),
		// HID interface
		descriptor.InterfaceHID.Bytes(),
		// HID class descriptor (will be patched with correct report length)
		func() []byte {
			classHID := descriptor.ClassHID.Bytes()
			// Update ClassLength to match our custom report descriptor
			classHID[7] = byte(len(CompositeHIDReportDescriptor))
			classHID[8] = byte(len(CompositeHIDReportDescriptor) >> 8)
			return classHID
		}(),
		descriptor.EndpointEP4IN.Bytes(),
		descriptor.EndpointEP5OUT.Bytes(),
	}),

	// HID report descriptors by interface number
	HID: map[uint16][]byte{
		usb.HID_INTERFACE: CompositeHIDReportDescriptor,
	},
}
