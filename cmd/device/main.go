package main

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"time"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"
	
	device "network-control-plane/api/proto"
)

type DeviceSimulator struct {
	deviceID   string
	token      string
	client     device.DeviceServiceClient
	log        *logrus.Logger
	deviceType device.DeviceType
	macAddress string
	serverAddr string
}

func main() {
	serverAddr := flag.String("server", "localhost:50051", "Control plane address")
	deviceType := flag.String("type", "router", "Device type: router, ap, gateway")
	hostnamePrefix := flag.String("hostname", "", "Hostname prefix (auto-generated if not provided)")
	flag.Parse()

	log := logrus.New()
	log.SetFormatter(&logrus.JSONFormatter{})

	// Connect to control plane
	conn, err := grpc.Dial(*serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	client := device.NewDeviceServiceClient(conn)

	// Create simulator
	sim := &DeviceSimulator{
		client:     client,
		log:        log,
		deviceType: parseDeviceType(*deviceType),
		macAddress: generateMAC(),
		serverAddr: *serverAddr,
	}

	if *hostnamePrefix == "" {
		*hostnamePrefix = fmt.Sprintf("device-%s", sim.macAddress[12:])
	}

	// Register device
	if err := sim.register(*hostnamePrefix); err != nil {
		log.Fatalf("Registration failed: %v", err)
	}

	// Start heartbeat loop
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start telemetry streaming
	go sim.streamTelemetry(ctx)

	// Run heartbeat loop
	sim.runHeartbeat(ctx)
}

func (s *DeviceSimulator) register(hostname string) error {
	ctx := context.Background()
	
	req := &device.RegisterRequest{
		MacAddress:      s.macAddress,
		IpAddress:       fmt.Sprintf("192.168.1.%d", rand.Intn(254)+2), // Skip .1 as it's often gateway
		Hostname:        hostname,
		Type:            s.deviceType,
		FirmwareVersion: fmt.Sprintf("v%d.%d.%d", rand.Intn(3)+1, rand.Intn(10), rand.Intn(10)),
		Metadata: map[string]string{
			"location": fmt.Sprintf("floor-%d", rand.Intn(5)+1),
			"zone":     fmt.Sprintf("zone-%c", 'A'+rand.Intn(5)),
		},
		Capabilities: []string{"wifi", "ethernet", "vpn"},
	}

	resp, err := s.client.Register(ctx, req)
	if err != nil {
		return fmt.Errorf("registration failed: %w", err)
	}

	s.deviceID = resp.DeviceId
	s.token = resp.Token

	s.log.WithFields(logrus.Fields{
		"device_id": s.deviceID,
		"mac":       s.macAddress,
		"type":      s.deviceType.String(),
		"server":    s.serverAddr,
	}).Info("Device registered successfully")

	return nil
}

func (s *DeviceSimulator) runHeartbeat(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.sendHeartbeat(); err != nil {
				s.log.WithError(err).Error("Heartbeat failed")
			}
		}
	}
}

func (s *DeviceSimulator) sendHeartbeat() error {
	ctx := context.Background()

	metrics := &device.DeviceMetrics{
		CpuUsagePercent:    float32(20 + rand.Float32()*60), // 20-80%
		MemoryUsagePercent: float32(30 + rand.Float32()*50), // 30-80%
		UptimeSeconds:      int64(time.Since(time.Now().Add(-time.Duration(rand.Intn(72)*int(time.Hour)))).Seconds()),
		BytesSent:          int64(rand.Int63n(1000000000)),
		BytesReceived:      int64(rand.Int63n(1000000000)),
		ConnectedClients:   int32(rand.Intn(50)),
		CustomMetrics: map[string]string{
			"temperature": fmt.Sprintf("%.2f", 45.0+rand.Float64()*20),
		},
	}

	req := &device.HeartbeatRequest{
		DeviceId: s.deviceID,
		Token:    s.token,
		Status:   device.DeviceStatus_DEVICE_STATUS_ONLINE,
		Metrics:  metrics,
	}

	resp, err := s.client.Heartbeat(ctx, req)
	if err != nil {
		return fmt.Errorf("heartbeat request failed: %w", err)
	}

	if !resp.Acknowledged {
		return fmt.Errorf("heartbeat not acknowledged")
	}

	s.log.WithField("device_id", s.deviceID).Debug("Heartbeat sent successfully")

	return nil
}

func (s *DeviceSimulator) streamTelemetry(ctx context.Context) {
	// Wait a bit before starting telemetry to allow registration
	time.Sleep(2 * time.Second)

	stream, err := s.client.StreamTelemetry(ctx)
	if err != nil {
		s.log.WithError(err).Error("Failed to create telemetry stream")
		return
	}

	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			telemetry := &device.TelemetryData{
				DeviceId:  s.deviceID,
				Timestamp: &timestamppb.Timestamp{Seconds: time.Now().Unix()},
				EventType: "periodic_telemetry",
				Data: map[string]string{
					"cpu_temp":      fmt.Sprintf("%.2f", 45.0+rand.Float64()*20),
					"signal_strength": fmt.Sprintf("%d", -50-rand.Intn(30)),
					"active_users":   fmt.Sprintf("%d", rand.Intn(100)),
				},
			}

			if err := stream.Send(telemetry); err != nil {
				s.log.WithError(err).Error("Failed to send telemetry")
				return
			}

			s.log.WithField("device_id", s.deviceID).Debug("Telemetry sent")
		}
	}
}

func parseDeviceType(t string) device.DeviceType {
	switch t {
	case "router":
		return device.DeviceType_DEVICE_TYPE_ROUTER
	case "ap":
		return device.DeviceType_DEVICE_TYPE_ACCESS_POINT
	case "gateway":
		return device.DeviceType_DEVICE_TYPE_GATEWAY
	case "switch":
		return device.DeviceType_DEVICE_TYPE_SWITCH
	case "iot":
		return device.DeviceType_DEVICE_TYPE_IOT_DEVICE
	default:
		return device.DeviceType_DEVICE_TYPE_ROUTER
	}
}

func generateMAC() string {
	return fmt.Sprintf("02:00:00:%02x:%02x:%02x",
		rand.Intn(256), rand.Intn(256), rand.Intn(256))
}