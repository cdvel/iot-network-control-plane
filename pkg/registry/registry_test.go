package registry_test

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	
	device "network-control-plane/api/proto"
	"network-control-plane/pkg/registry"
)

func TestRegisterDevice(t *testing.T) {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel) // Suppress logs during tests
	reg := registry.NewRegistry(log)
	
	req := &device.RegisterRequest{
		MacAddress:      "00:11:22:33:44:55",
		IpAddress:       "192.168.1.100",
		Hostname:        "test-router",
		Type:            device.DeviceType_DEVICE_TYPE_ROUTER,
		FirmwareVersion: "v1.0.0",
	}
	
	registeredDevice, err := reg.Register(context.Background(), req)
	
	assert.NoError(t, err)
	assert.NotEmpty(t, registeredDevice.ID)
	assert.Equal(t, req.MacAddress, registeredDevice.MACAddress)
	assert.Equal(t, device.DeviceStatus_DEVICE_STATUS_ONLINE, registeredDevice.Status)
	assert.Equal(t, req.Type, registeredDevice.Type)
	assert.Equal(t, req.Hostname, registeredDevice.Hostname)
}

func TestHeartbeatUpdate(t *testing.T) {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel) // Suppress logs during tests
	reg := registry.NewRegistry(log)
	
	// Register device first
	testDevice, _ := reg.Register(context.Background(), &device.RegisterRequest{
		MacAddress: "00:11:22:33:44:55",
		Type:       device.DeviceType_DEVICE_TYPE_ROUTER,
	})
	
	// Update heartbeat
	metrics := &device.DeviceMetrics{
		CpuUsagePercent:    45.5,
		MemoryUsagePercent: 60.0,
		UptimeSeconds:      3600,
		BytesSent:          1000000,
		BytesReceived:      2000000,
		ConnectedClients:   10,
	}
	
	err := reg.UpdateHeartbeat(testDevice.ID, device.DeviceStatus_DEVICE_STATUS_ONLINE, metrics)
	
	assert.NoError(t, err)
	
	updatedDevice, _ := reg.GetDevice(testDevice.ID)
	assert.Equal(t, float32(45.5), updatedDevice.LastMetrics.CpuUsagePercent)
	assert.Equal(t, float32(60.0), updatedDevice.LastMetrics.MemoryUsagePercent)
	assert.Equal(t, device.DeviceStatus_DEVICE_STATUS_ONLINE, updatedDevice.Status)
	assert.WithinDuration(t, time.Now(), updatedDevice.LastHeartbeat, 1*time.Second)
}

func TestGetDevice(t *testing.T) {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel) // Suppress logs during tests
	reg := registry.NewRegistry(log)
	
	// Register device
	testDevice, _ := reg.Register(context.Background(), &device.RegisterRequest{
		MacAddress: "00:11:22:33:44:55",
		Type:       device.DeviceType_DEVICE_TYPE_ROUTER,
	})
	
	// Get device
	retrieved, err := reg.GetDevice(testDevice.ID)
	
	assert.NoError(t, err)
	assert.Equal(t, testDevice.ID, retrieved.ID)
	assert.Equal(t, testDevice.MACAddress, retrieved.MACAddress)
}

func TestGetDeviceNotFound(t *testing.T) {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel) // Suppress logs during tests
	reg := registry.NewRegistry(log)
	
	_, err := reg.GetDevice("non-existent-id")
	
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "device not found")
}

func TestListDevices(t *testing.T) {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel) // Suppress logs during tests
	reg := registry.NewRegistry(log)
	
	// Register multiple devices
	_, _ = reg.Register(context.Background(), &device.RegisterRequest{
		MacAddress: "00:11:22:33:44:55",
		Type:       device.DeviceType_DEVICE_TYPE_ROUTER,
		Hostname:   "router-1",
	})
	
	_, _ = reg.Register(context.Background(), &device.RegisterRequest{
		MacAddress: "00:11:22:33:44:56",
		Type:       device.DeviceType_DEVICE_TYPE_ACCESS_POINT,
		Hostname:   "ap-1",
	})
	
	// List all devices
	allDevices := reg.ListDevices(nil)
	assert.Equal(t, 2, len(allDevices))
	
	// List with filter (only routers)
	routerFilter := func(d *registry.Device) bool {
		return d.Type == device.DeviceType_DEVICE_TYPE_ROUTER
	}
	routers := reg.ListDevices(routerFilter)
	assert.Equal(t, 1, len(routers))
	assert.Equal(t, "router-1", routers[0].Hostname)
}

func TestDeregister(t *testing.T) {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel) // Suppress logs during tests
	reg := registry.NewRegistry(log)
	
	// Register device
	testDevice, _ := reg.Register(context.Background(), &device.RegisterRequest{
		MacAddress: "00:11:22:33:44:55",
		Type:       device.DeviceType_DEVICE_TYPE_ROUTER,
	})
	
	// Verify device exists
	_, err := reg.GetDevice(testDevice.ID)
	assert.NoError(t, err)
	
	// Deregister device
	err = reg.Deregister(testDevice.ID)
	assert.NoError(t, err)
	
	// Verify device no longer exists
	_, err = reg.GetDevice(testDevice.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "device not found")
}

func TestDeviceReRegistration(t *testing.T) {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel) // Suppress logs during tests
	reg := registry.NewRegistry(log)
	
	// Register device first time
	req1 := &device.RegisterRequest{
		MacAddress:      "00:11:22:33:44:55",
		IpAddress:       "192.168.1.100",
		Hostname:        "test-router",
		Type:            device.DeviceType_DEVICE_TYPE_ROUTER,
		FirmwareVersion: "v1.0.0",
	}
	
	device1, err := reg.Register(context.Background(), req1)
	assert.NoError(t, err)
	
	// Register same MAC again (re-registration)
	req2 := &device.RegisterRequest{
		MacAddress:      "00:11:22:33:44:55", // Same MAC
		IpAddress:       "192.168.1.101",     // Different IP
		Hostname:        "test-router-updated",
		Type:            device.DeviceType_DEVICE_TYPE_ROUTER,
		FirmwareVersion: "v1.0.1",
	}
	
	device2, err := reg.Register(context.Background(), req2)
	assert.NoError(t, err)
	
	// Should be the same device ID
	assert.Equal(t, device1.ID, device2.ID)
	// Should have updated information
	assert.Equal(t, "192.168.1.101", device2.IPAddress)
	assert.Equal(t, "test-router-updated", device2.Hostname)
	assert.Equal(t, "v1.0.1", device2.FirmwareVersion)
}