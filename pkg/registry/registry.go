package registry

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	device "network-control-plane/api/proto"
)

// Device represents a registered network device
type Device struct {
	ID              string
	MACAddress      string
	IPAddress       string
	Hostname        string
	Type            device.DeviceType
	FirmwareVersion string
	Status          device.DeviceStatus
	Metadata        map[string]string
	Capabilities    []string
	RegisteredAt    time.Time
	LastHeartbeat   time.Time
	LastMetrics     *device.DeviceMetrics
	Token           string
	HealthHistory   []HealthRecord
}

type HealthRecord struct {
	Timestamp time.Time
	Status    device.DeviceStatus
	Metrics   *device.DeviceMetrics
}

// Registry manages device lifecycle
type Registry struct {
	devices map[string]*Device
	mu      sync.RWMutex
	log     *logrus.Logger
}

func NewRegistry(log *logrus.Logger) *Registry {
	return &Registry{
		devices: make(map[string]*Device),
		log:     log,
	}
}

// Register adds a new device
func (r *Registry) Register(ctx context.Context, req *device.RegisterRequest) (*Device, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if device already exists by MAC
	for _, existingDevice := range r.devices {
		if existingDevice.MACAddress == req.MacAddress {
			r.log.WithField("mac", req.MacAddress).Info("Device re-registering")
			// Update existing device
			existingDevice.IPAddress = req.IpAddress
			existingDevice.Hostname = req.Hostname
			existingDevice.FirmwareVersion = req.FirmwareVersion
			existingDevice.LastHeartbeat = time.Now()
			existingDevice.Status = device.DeviceStatus_DEVICE_STATUS_ONLINE
			return existingDevice, nil
		}
	}

	// Create new device
	newDevice := &Device{
		ID:              uuid.New().String(),
		MACAddress:      req.MacAddress,
		IPAddress:       req.IpAddress,
		Hostname:        req.Hostname,
		Type:            req.Type,
		FirmwareVersion: req.FirmwareVersion,
		Status:          device.DeviceStatus_DEVICE_STATUS_ONLINE,
		Metadata:        req.Metadata,
		Capabilities:    req.Capabilities,
		RegisteredAt:    time.Now(),
		LastHeartbeat:   time.Now(),
		Token:           generateToken(),
		HealthHistory:   make([]HealthRecord, 0, 100),
	}

	r.devices[newDevice.ID] = newDevice
	
	r.log.WithFields(logrus.Fields{
		"device_id": newDevice.ID,
		"mac":       newDevice.MACAddress,
		"type":      newDevice.Type.String(),
	}).Info("Device registered")

	return newDevice, nil
}

// UpdateHeartbeat updates device health status
func (r *Registry) UpdateHeartbeat(deviceID string, status device.DeviceStatus, metrics *device.DeviceMetrics) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	device, exists := r.devices[deviceID]
	if !exists {
		return fmt.Errorf("device not found: %s", deviceID)
	}

	device.LastHeartbeat = time.Now()
	device.Status = status
	device.LastMetrics = metrics

	// Add to health history (keep last 100 records)
	record := HealthRecord{
		Timestamp: time.Now(),
		Status:    status,
		Metrics:   metrics,
	}
	
	device.HealthHistory = append(device.HealthHistory, record)
	if len(device.HealthHistory) > 100 {
		device.HealthHistory = device.HealthHistory[1:]
	}

	r.log.WithFields(logrus.Fields{
		"device_id": deviceID,
		"status":    status.String(),
	}).Debug("Heartbeat updated")

	return nil
}

// GetDevice retrieves a device by ID
func (r *Registry) GetDevice(deviceID string) (*Device, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	device, exists := r.devices[deviceID]
	if !exists {
		return nil, fmt.Errorf("device not found: %s", deviceID)
	}

	return device, nil
}

// ListDevices returns all devices with optional filtering
func (r *Registry) ListDevices(filter func(*Device) bool) []*Device {
	r.mu.RLock()
	defer r.mu.RUnlock()

	devices := make([]*Device, 0)
	for _, d := range r.devices {
		if filter == nil || filter(d) {
			devices = append(devices, d)
		}
	}

	return devices
}

// GetStats returns registry statistics
func (r *Registry) GetStats() map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := map[string]interface{}{
		"total_devices": len(r.devices),
		"by_status":     make(map[string]int),
		"by_type":       make(map[string]int),
	}

	statusCounts := make(map[string]int)
	typeCounts := make(map[string]int)

	for _, device := range r.devices {
		statusCounts[device.Status.String()]++
		typeCounts[device.Type.String()]++
	}

	stats["by_status"] = statusCounts
	stats["by_type"] = typeCounts

	return stats
}

// Deregister removes a device
func (r *Registry) Deregister(deviceID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.devices[deviceID]; !exists {
		return fmt.Errorf("device not found: %s", deviceID)
	}

	delete(r.devices, deviceID)
	
	r.log.WithField("device_id", deviceID).Info("Device deregistered")

	return nil
}

func generateToken() string {
	return uuid.New().String()
}