package telemetry

import (
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	device "network-control-plane/api/proto"
)

// Collector receives and processes telemetry data from devices
type Collector struct {
	telemetryBuffer map[string][]*device.TelemetryData
	bufferMutex     sync.RWMutex
	bufferSize      int
	log             *logrus.Logger
}

func NewCollector(log *logrus.Logger) *Collector {
	c := &Collector{
		telemetryBuffer: make(map[string][]*device.TelemetryData),
		bufferSize:      100, // Keep last 100 telemetry entries per device
		log:             log,
	}
	
	return c
}

// Collect receives telemetry data from a device
func (c *Collector) Collect(deviceID string, data *device.TelemetryData) error {
	c.bufferMutex.Lock()
	defer c.bufferMutex.Unlock()
	
	if _, exists := c.telemetryBuffer[deviceID]; !exists {
		c.telemetryBuffer[deviceID] = make([]*device.TelemetryData, 0, c.bufferSize)
	}
	
	// Add new telemetry data
	c.telemetryBuffer[deviceID] = append(c.telemetryBuffer[deviceID], data)
	
	// Keep only the last N entries
	if len(c.telemetryBuffer[deviceID]) > c.bufferSize {
		c.telemetryBuffer[deviceID] = c.telemetryBuffer[deviceID][len(c.telemetryBuffer[deviceID])-c.bufferSize:]
	}
	
	c.log.WithFields(logrus.Fields{
		"device_id":  deviceID,
		"event_type": data.EventType,
	}).Debug("Telemetry collected")
	
	return nil
}

// GetTelemetryForDevice returns telemetry data for a specific device
func (c *Collector) GetTelemetryForDevice(deviceID string) []*device.TelemetryData {
	c.bufferMutex.RLock()
	defer c.bufferMutex.RUnlock()
	
	if data, exists := c.telemetryBuffer[deviceID]; exists {
		result := make([]*device.TelemetryData, len(data))
		copy(result, data)
		return result
	}
	
	return []*device.TelemetryData{}
}

// GetRecentTelemetry returns telemetry for all devices within a time window
func (c *Collector) GetRecentTelemetry(since time.Time) map[string][]*device.TelemetryData {
	c.bufferMutex.RLock()
	defer c.bufferMutex.RUnlock()
	
	result := make(map[string][]*device.TelemetryData)
	
	for deviceID, data := range c.telemetryBuffer {
		recentData := make([]*device.TelemetryData, 0)
		for _, entry := range data {
			if entry.Timestamp != nil && entry.Timestamp.Seconds >= since.Unix() {
				recentData = append(recentData, entry)
			}
		}
		if len(recentData) > 0 {
			result[deviceID] = recentData
		}
	}
	
	return result
}