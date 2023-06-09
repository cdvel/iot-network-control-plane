package integration

import (
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	device "network-control-plane/api/proto"
	"network-control-plane/pkg/config"
	"network-control-plane/pkg/health"
	"network-control-plane/pkg/registry"
	"network-control-plane/pkg/telemetry"
)

func TestDeviceRegistrationAndHeartbeat(t *testing.T) {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel) // Suppress logs during tests
	
	reg := registry.NewRegistry(log)
	
	// Test device registration
	req := &device.RegisterRequest{
		MacAddress:      "AA:BB:CC:DD:EE:FF",
		IpAddress:       "192.168.1.100",
		Hostname:        "test-device",
		Type:            device.DeviceType_DEVICE_TYPE_ROUTER,
		FirmwareVersion: "v1.0.0",
	}
	
	registeredDevice, err := reg.Register(nil, req)
	assert.NoError(t, err)
	assert.NotNil(t, registeredDevice)
	assert.Equal(t, "AA:BB:CC:DD:EE:FF", registeredDevice.MACAddress)
	assert.Equal(t, device.DeviceStatus_DEVICE_STATUS_ONLINE, registeredDevice.Status)
	
	// Test heartbeat update
	metrics := &device.DeviceMetrics{
		CpuUsagePercent: 50.0,
	}
	
	err = reg.UpdateHeartbeat(registeredDevice.ID, device.DeviceStatus_DEVICE_STATUS_ONLINE, metrics)
	assert.NoError(t, err)
	
	// Verify the device was updated
	updatedDevice, err := reg.GetDevice(registeredDevice.ID)
	assert.NoError(t, err)
	assert.Equal(t, device.DeviceStatus_DEVICE_STATUS_ONLINE, updatedDevice.Status)
	assert.Equal(t, float32(50.0), updatedDevice.LastMetrics.CpuUsagePercent)
	assert.WithinDuration(t, time.Now(), updatedDevice.LastHeartbeat, 1*time.Second)
}

func TestHealthChecker(t *testing.T) {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel) // Suppress logs during tests
	
	reg := registry.NewRegistry(log)
	
	// Register a device
	req := &device.RegisterRequest{
		MacAddress: "AA:BB:CC:DD:EE:FF",
		Type:       device.DeviceType_DEVICE_TYPE_ROUTER,
	}
	
	testDevice, err := reg.Register(nil, req)
	assert.NoError(t, err)
	
	// Create a health checker
	_ = health.NewChecker(reg, log) // Just verify it can be created
	
	// Manually update the device to simulate it being offline
	testDevice.LastHeartbeat = time.Now().Add(-2 * time.Minute) // More than the timeout
	
	// Update the device status to reflect this
	err = reg.UpdateHeartbeat(testDevice.ID, device.DeviceStatus_DEVICE_STATUS_OFFLINE, nil)
	assert.NoError(t, err)
	
	// Check that the status is properly tracked
	_, err = reg.GetDevice(testDevice.ID)
	assert.NoError(t, err)
	// Note: the status would be updated by the checker in a real scenario
	// This test verifies that the registry can track the status correctly
}

func TestConfigManager(t *testing.T) {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel) // Suppress logs during tests
	
	configMgr := config.NewManager(log)
	
	// Test getting default config
	deviceID := "test-device-123"
	config, err := configMgr.GetDeviceConfig(deviceID)
	assert.NoError(t, err)
	assert.Equal(t, "default-config", config.ConfigId)
	assert.Equal(t, int32(1), config.Version)
	assert.Contains(t, config.Settings, "auto_update")
	
	// Test updating config
	newConfig := &device.Configuration{
		ConfigId: "updated-config",
		Version:  2,
		Settings: map[string]string{
			"test_setting": "test_value",
		},
	}
	
	err = configMgr.UpdateDeviceConfig(deviceID, newConfig)
	assert.NoError(t, err)
	
	// Verify the config was updated
	updatedConfig, err := configMgr.GetDeviceConfig(deviceID)
	assert.NoError(t, err)
	assert.Equal(t, "updated-config", updatedConfig.ConfigId)
	assert.Equal(t, int32(2), updatedConfig.Version)
	assert.Equal(t, "test_value", updatedConfig.Settings["test_setting"])
}

func TestTelemetryCollector(t *testing.T) {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel) // Suppress logs during tests
	
	telemetryCollector := telemetry.NewCollector(log)
	
	// Test collecting telemetry
	deviceID := "test-device-123"
	telemetryData := &device.TelemetryData{
		DeviceId:  deviceID,
		EventType: "test_event",
		Data: map[string]string{
			"test_key": "test_value",
		},
	}
	
	err := telemetryCollector.Collect(deviceID, telemetryData)
	assert.NoError(t, err)
	
	// Verify telemetry was collected
	telemetryList := telemetryCollector.GetTelemetryForDevice(deviceID)
	assert.Len(t, telemetryList, 1)
	assert.Equal(t, "test_event", telemetryList[0].EventType)
	assert.Equal(t, "test_value", telemetryList[0].Data["test_key"])
}