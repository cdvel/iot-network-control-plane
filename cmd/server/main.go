package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"

	device "network-control-plane/api/proto"
	"network-control-plane/internal/metrics"
	"network-control-plane/pkg/config"
	"network-control-plane/pkg/health"
	"network-control-plane/pkg/registry"
	"network-control-plane/pkg/server"
	"network-control-plane/pkg/telemetry"
)

func main() {
	port := flag.Int("port", 50051, "gRPC server port")
	restPort := flag.Int("rest-port", 8080, "REST server port")
	flag.Parse()

	log := logrus.New()
	log.SetFormatter(&logrus.JSONFormatter{})
	log.SetLevel(logrus.InfoLevel)

	// Initialize components
	deviceRegistry := registry.NewRegistry(log)
	healthChecker := health.NewChecker(deviceRegistry, log)
	configManager := config.NewManager(log)
	telemetryCollector := telemetry.NewCollector(log)

	// Start health check monitor
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go healthChecker.Start(ctx)

	// gRPC server
	grpcServer := grpc.NewServer(
		grpc.MaxConcurrentStreams(1000),
		grpc.KeepaliveParams(keepalive.ServerParameters{
			Time:    30 * time.Second,
			Timeout: 10 * time.Second,
		}),
	)

	deviceService := server.NewDeviceServer(deviceRegistry, healthChecker, configManager, telemetryCollector, log)
	device.RegisterDeviceServiceServer(grpcServer, deviceService)
	
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	// Start gRPC server
	go func() {
		log.Infof("Starting gRPC server on :%d", *port)
		if err := grpcServer.Serve(listener); err != nil {
			log.Fatalf("gRPC server error: %v", err)
		}
	}()

	// REST API server
	restServer := server.NewRESTServer(deviceRegistry, healthChecker, configManager, telemetryCollector, log)
	httpMux := http.NewServeMux()
	httpMux.HandleFunc("/api/v1/devices", restServer.ListDevices)
	httpMux.HandleFunc("/api/v1/devices/", restServer.GetDevice)
	httpMux.HandleFunc("/api/v1/stats", restServer.GetStats)
	httpMux.Handle("/metrics", metrics.Handler())
	httpMux.HandleFunc("/health", restServer.Health)
	
	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", *restPort),
		Handler: httpMux,
	}
	
	go func() {
		log.Infof("Starting REST server on :%d", *restPort)
		if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	// Graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Info("Shutting down servers...")
	
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()
	
	grpcServer.GracefulStop()
	httpServer.Shutdown(shutdownCtx)
	cancel() // Stop health checker
	
	log.Info("Shutdown complete")
}