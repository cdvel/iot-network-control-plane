package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/types/known/timestamppb"

	device "network-control-plane/api/proto"
	"network-control-plane/pkg/config"
	"network-control-plane/pkg/health"
	"network-control-plane/pkg/registry"
	"network-control-plane/pkg/telemetry"
)

// DeviceServer implements the DeviceService gRPC interface
type DeviceServer struct {
	device.UnimplementedDeviceServiceServer
	registry  *registry.Registry
	health    *health.Checker
	config    *config.Manager
	telemetry *telemetry.Collector
	log       *logrus.Logger
}

func NewDeviceServer(registry *registry.Registry, health *health.Checker, config *config.Manager, telemetry *telemetry.Collector, log *logrus.Logger) *DeviceServer {
	return &DeviceServer{
		registry:  registry,
		health:    health,
		config:    config,
		telemetry: telemetry,
		log:       log,
	}
}

// Register registers a new device with the control plane
func (s *DeviceServer) Register(ctx context.Context, req *device.RegisterRequest) (*device.RegisterResponse, error) {
	s.log.WithFields(logrus.Fields{
		"mac_address": req.MacAddress,
		"ip_address":  req.IpAddress,
		"type":        req.Type.String(),
	}).Info("Device registration request")

	registeredDevice, err := s.registry.Register(ctx, req)
	if err != nil {
		s.log.WithError(err).Error("Failed to register device")
		return nil, err
	}

	resp := &device.RegisterResponse{
		DeviceId:                 registeredDevice.ID,
		Token:                    registeredDevice.Token,
		HeartbeatIntervalSeconds: 30,
		RegisteredAt:             &timestamppb.Timestamp{Seconds: registeredDevice.RegisteredAt.Unix()},
	}

	s.log.WithFields(logrus.Fields{
		"device_id": registeredDevice.ID,
		"token":     registeredDevice.Token,
	}).Info("Device registered successfully")

	return resp, nil
}

// Heartbeat updates device status and metrics
func (s *DeviceServer) Heartbeat(ctx context.Context, req *device.HeartbeatRequest) (*device.HeartbeatResponse, error) {
	s.log.WithFields(logrus.Fields{
		"device_id": req.DeviceId,
		"status":    req.Status.String(),
	}).Debug("Heartbeat received")

	// Verify token
	// In a real implementation, we'd validate the token here
	// For now, we'll just proceed assuming the token is valid

	err := s.registry.UpdateHeartbeat(req.DeviceId, req.Status, req.Metrics)
	if err != nil {
		s.log.WithError(err).WithField("device_id", req.DeviceId).Error("Failed to update heartbeat")
		return nil, err
	}

	resp := &device.HeartbeatResponse{
		Acknowledged:           true,
		NextHeartbeatSeconds: 30,
	}

	return resp, nil
}

// StreamTelemetry receives telemetry data from devices
func (s *DeviceServer) StreamTelemetry(stream device.DeviceService_StreamTelemetryServer) error {
	s.log.Info("Starting telemetry stream")

	for {
		data, err := stream.Recv()
		if err != nil {
			s.log.WithError(err).Info("Telemetry stream ended")
			return nil
		}

		s.log.WithFields(logrus.Fields{
			"device_id":  data.DeviceId,
			"event_type": data.EventType,
		}).Debug("Telemetry received")

		// Collect telemetry data
		if err := s.telemetry.Collect(data.DeviceId, data); err != nil {
			s.log.WithError(err).WithField("device_id", data.DeviceId).Error("Failed to collect telemetry")
		}
	}
}

// GetConfiguration streams configuration updates to the device
func (s *DeviceServer) GetConfiguration(req *device.ConfigRequest, stream device.DeviceService_GetConfigurationServer) error {
	s.log.WithField("device_id", req.DeviceId).Info("Configuration stream requested")

	// Verify token
	// In a real implementation, we'd validate the token here

	// Get configuration for the device
	config, err := s.config.GetDeviceConfig(req.DeviceId)
	if err != nil {
		s.log.WithError(err).WithField("device_id", req.DeviceId).Error("Failed to get device configuration")
		return err
	}

	if err := stream.Send(config); err != nil {
		s.log.WithError(err).Error("Failed to send configuration")
		return err
	}

	// In a real implementation, we'd keep the stream open and send updates
	// when configuration changes. For now, we'll just send the initial config
	// and close the stream.

	return nil
}

// ExecuteCommand executes a command on the device
func (s *DeviceServer) ExecuteCommand(ctx context.Context, req *device.Command) (*device.CommandResponse, error) {
	s.log.WithFields(logrus.Fields{
		"command_id": req.CommandId,
		"device_id":  req.DeviceId,
		"type":       req.Type.String(),
	}).Info("Command execution requested")

	// Verify device exists
	_, err := s.registry.GetDevice(req.DeviceId)
	if err != nil {
		s.log.WithError(err).WithField("device_id", req.DeviceId).Error("Device not found")
		return nil, err
	}

	// Execute the command based on type
	var success bool
	var message string

	switch req.Type {
	case device.CommandType_COMMAND_TYPE_REBOOT:
		message = "Reboot command queued"
		success = true
	case device.CommandType_COMMAND_TYPE_UPDATE_FIRMWARE:
		message = "Firmware update command queued"
		success = true
	case device.CommandType_COMMAND_TYPE_RUN_DIAGNOSTICS:
		message = "Diagnostics command queued"
		success = true
	case device.CommandType_COMMAND_TYPE_RESET_CONFIG:
		message = "Config reset command queued"
		success = true
	default:
		message = "Unknown command type"
		success = false
	}

	resp := &device.CommandResponse{
		CommandId: req.CommandId,
		Success:   success,
		Message:   message,
		Result:    map[string]string{},
	}

	return resp, nil
}

// Deregister removes a device from the registry
func (s *DeviceServer) Deregister(ctx context.Context, req *device.DeregisterRequest) (*device.DeregisterResponse, error) {
	s.log.WithField("device_id", req.DeviceId).Info("Device deregistration requested")

	// Verify token
	// In a real implementation, we'd validate the token here

	err := s.registry.Deregister(req.DeviceId)
	if err != nil {
		s.log.WithError(err).WithField("device_id", req.DeviceId).Error("Failed to deregister device")
		return nil, err
	}

	resp := &device.DeregisterResponse{
		Acknowledged: true,
	}

	return resp, nil
}

// RESTServer provides HTTP REST endpoints
type RESTServer struct {
	registry  *registry.Registry
	health    *health.Checker
	config    *config.Manager
	telemetry *telemetry.Collector
	log       *logrus.Logger
}

func NewRESTServer(registry *registry.Registry, health *health.Checker, config *config.Manager, telemetry *telemetry.Collector, log *logrus.Logger) *RESTServer {
	return &RESTServer{
		registry:  registry,
		health:    health,
		config:    config,
		telemetry: telemetry,
		log:       log,
	}
}

// ListDevices returns all devices
func (s *RESTServer) ListDevices(w http.ResponseWriter, r *http.Request) {
	devices := s.registry.ListDevices(nil)

	response := make([]map[string]interface{}, 0, len(devices))
	for _, d := range devices {
		deviceMap := map[string]interface{}{
			"id":               d.ID,
			"mac_address":      d.MACAddress,
			"ip_address":       d.IPAddress,
			"hostname":         d.Hostname,
			"type":             d.Type.String(),
			"firmware_version": d.FirmwareVersion,
			"status":           d.Status.String(),
			"registered_at":    d.RegisteredAt.Format(time.RFC3339),
			"last_heartbeat":   d.LastHeartbeat.Format(time.RFC3339),
			"metadata":         d.Metadata,
			"capabilities":     d.Capabilities,
		}

		if d.LastMetrics != nil {
			deviceMap["last_metrics"] = map[string]interface{}{
				"cpu_usage_percent":    d.LastMetrics.CpuUsagePercent,
				"memory_usage_percent": d.LastMetrics.MemoryUsagePercent,
				"uptime_seconds":       d.LastMetrics.UptimeSeconds,
				"bytes_sent":           d.LastMetrics.BytesSent,
				"bytes_received":       d.LastMetrics.BytesReceived,
				"connected_clients":    d.LastMetrics.ConnectedClients,
			}
		}

		response = append(response, deviceMap)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetDevice returns a specific device by ID
func (s *RESTServer) GetDevice(w http.ResponseWriter, r *http.Request) {
	deviceID := r.URL.Path[len("/api/v1/devices/"):] // Extract device ID from URL

	device, err := s.registry.GetDevice(deviceID)
	if err != nil {
		http.Error(w, "Device not found", http.StatusNotFound)
		return
	}

	deviceMap := map[string]interface{}{
		"id":               device.ID,
		"mac_address":      device.MACAddress,
		"ip_address":       device.IPAddress,
		"hostname":         device.Hostname,
		"type":             device.Type.String(),
		"firmware_version": device.FirmwareVersion,
		"status":           device.Status.String(),
		"registered_at":    device.RegisteredAt.Format(time.RFC3339),
		"last_heartbeat":   device.LastHeartbeat.Format(time.RFC3339),
		"metadata":         device.Metadata,
		"capabilities":     device.Capabilities,
		"health_history":   device.HealthHistory,
	}

	if device.LastMetrics != nil {
		deviceMap["last_metrics"] = map[string]interface{}{
			"cpu_usage_percent":    device.LastMetrics.CpuUsagePercent,
			"memory_usage_percent": device.LastMetrics.MemoryUsagePercent,
			"uptime_seconds":       device.LastMetrics.UptimeSeconds,
			"bytes_sent":           device.LastMetrics.BytesSent,
			"bytes_received":       device.LastMetrics.BytesReceived,
			"connected_clients":    device.LastMetrics.ConnectedClients,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(deviceMap)
}

// GetStats returns fleet statistics
func (s *RESTServer) GetStats(w http.ResponseWriter, r *http.Request) {
	stats := s.registry.GetStats()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// Health returns control plane health status
func (s *RESTServer) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, `{"status": "healthy", "timestamp": "`+time.Now().Format(time.RFC3339)+`"}`)
}