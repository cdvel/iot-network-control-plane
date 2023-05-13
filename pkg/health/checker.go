package health

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
	
	device "network-control-plane/api/proto"
	"network-control-plane/pkg/registry"
)

// Checker monitors device health
type Checker struct {
	registry *registry.Registry
	log      *logrus.Logger
	interval time.Duration
	timeout  time.Duration
}

func NewChecker(reg *registry.Registry, log *logrus.Logger) *Checker {
	return &Checker{
		registry: reg,
		log:      log,
		interval: 30 * time.Second,
		timeout:  90 * time.Second,
	}
}

// Start begins the health check loop
func (c *Checker) Start(ctx context.Context) {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	c.log.Info("Health checker started")

	for {
		select {
		case <-ctx.Done():
			c.log.Info("Health checker stopped")
			return
		case <-ticker.C:
			c.checkDevices()
		}
	}
}

func (c *Checker) checkDevices() {
	devices := c.registry.ListDevices(nil)
	now := time.Now()

	for _, d := range devices {
		timeSinceHeartbeat := now.Sub(d.LastHeartbeat)

		var newStatus device.DeviceStatus
		switch {
		case timeSinceHeartbeat > c.timeout:
			newStatus = device.DeviceStatus_DEVICE_STATUS_OFFLINE
		case timeSinceHeartbeat > c.interval*2:
			newStatus = device.DeviceStatus_DEVICE_STATUS_DEGRADED
		default:
			continue // Device is healthy
		}

		if d.Status != newStatus {
			c.log.WithFields(logrus.Fields{
				"device_id":  d.ID,
				"old_status": d.Status.String(),
				"new_status": newStatus.String(),
				"time_since": timeSinceHeartbeat.String(),
			}).Warn("Device status changed")

			c.registry.UpdateHeartbeat(d.ID, newStatus, d.LastMetrics)
		}
	}
}