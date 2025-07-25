package main

import (
	"encoding/json"
	"log"
	"math"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// StatusServer tracks the status of zone transfers and provides HTTP endpoints
type StatusServer struct {
	startTime    time.Time
	totalZones   uint32
	completed    uint32
	failed       uint32
	active       sync.Map // map[string]time.Time - active zone transfers
	activeCount  uint32
	mu           sync.RWMutex
	recentFailed []string // recent failures for debugging
}

// StatusResponse represents the JSON response for status endpoint
type StatusResponse struct {
	StartTime    time.Time `json:"start_time"`
	Runtime      string    `json:"runtime"`
	TotalZones   uint32    `json:"total_zones"`
	Completed    uint32    `json:"completed"`
	Failed       uint32    `json:"failed"`
	Active       uint32    `json:"active"`
	Remaining    uint32    `json:"remaining"`
	SuccessRate  float64   `json:"success_rate"`
	TransferRate float64   `json:"transfer_rate_per_minute"`
	RecentFailed []string  `json:"recent_failed,omitempty"`
}

// HealthResponse represents the JSON response for health endpoint
type HealthResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

var statusServer *StatusServer

// NewStatusServer creates a new status server instance
func NewStatusServer() *StatusServer {
	return &StatusServer{
		startTime:    time.Now(),
		totalZones:   0, // Will be updated as domains are discovered
		recentFailed: make([]string, 0),
	}
}

// IncrementTotalZones increments the total zone count as domains are discovered
func (s *StatusServer) IncrementTotalZones(change uint32) {
	atomic.AddUint32(&s.totalZones, change)
}

// StartTransfer marks a zone as actively being transferred
func (s *StatusServer) StartTransfer(zone string) {
	s.active.Store(zone, time.Now())
	atomic.AddUint32(&s.activeCount, 1)
}

// CompleteTransfer marks a zone transfer as completed
func (s *StatusServer) CompleteTransfer(zone string) {
	// Only process if the zone is still active (avoid double-counting)
	if _, exists := s.active.LoadAndDelete(zone); exists {
		atomic.AddUint32(&s.activeCount, ^uint32(0)) // decrement
		atomic.AddUint32(&s.completed, 1)
	}
}

// FailTransfer marks a zone transfer as failed
func (s *StatusServer) FailTransfer(zone string, reason string) {
	// Only process if the zone is still active (avoid double-counting)
	if _, exists := s.active.LoadAndDelete(zone); exists {
		atomic.AddUint32(&s.activeCount, ^uint32(0)) // decrement
		atomic.AddUint32(&s.failed, 1)

		// Add to recent failures (keep last 10)
		s.mu.Lock()
		failureEntry := zone
		if reason != "" {
			failureEntry += ": " + reason
		}
		s.recentFailed = append(s.recentFailed, failureEntry)
		if len(s.recentFailed) > 10 {
			s.recentFailed = s.recentFailed[1:]
		}
		s.mu.Unlock()
	}
}

// GetStatus returns current status information
func (s *StatusServer) GetStatus() StatusResponse {
	s.mu.RLock()
	recentFailed := make([]string, len(s.recentFailed))
	copy(recentFailed, s.recentFailed)
	s.mu.RUnlock()

	completed := atomic.LoadUint32(&s.completed)
	failed := atomic.LoadUint32(&s.failed)
	active := atomic.LoadUint32(&s.activeCount)
	totalZones := atomic.LoadUint32(&s.totalZones)

	runtime := time.Since(s.startTime)
	remaining := uint32(0)
	if totalZones > completed+failed {
		remaining = totalZones - completed - failed
	}

	var successRate float64
	if completed+failed > 0 {
		successRate = math.Round(float64(completed)/float64(completed+failed)*100*100) / 100
	}

	var transferRate float64
	if runtime.Minutes() > 0 {
		transferRate = math.Round(float64(completed)/runtime.Minutes()*100) / 100
	}

	return StatusResponse{
		StartTime:    s.startTime,
		Runtime:      runtime.Round(time.Second).String(),
		TotalZones:   totalZones,
		Completed:    completed,
		Failed:       failed,
		Active:       active,
		Remaining:    remaining,
		SuccessRate:  successRate,
		TransferRate: transferRate,
		RecentFailed: recentFailed,
	}
}

// HTTP Handlers

func statusHandler(w http.ResponseWriter, r *http.Request) {
	if statusServer == nil {
		http.Error(w, "Status server not initialized", http.StatusInternalServerError)
		return
	}

	status := statusServer.GetStatus()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	health := HealthResponse{
		Status:  "ok",
		Message: "ALLXFR is running",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}

func progressHandler(w http.ResponseWriter, r *http.Request) {
	if statusServer == nil {
		http.Error(w, "Status server not initialized", http.StatusInternalServerError)
		return
	}

	status := statusServer.GetStatus()

	// Simplified progress response
	attempted := status.Completed + status.Failed
	var percentage float64
	if status.TotalZones > 0 {
		percentage = math.Round(float64(attempted)/float64(status.TotalZones)*100*100) / 100
	}

	progress := map[string]interface{}{
		"completed":  status.Completed,
		"failed":     status.Failed,
		"attempted":  attempted,
		"total":      status.TotalZones,
		"remaining":  status.Remaining,
		"active":     status.Active,
		"percentage": percentage,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(progress)
}

// StartStatusServer starts the HTTP status server in a separate goroutine
func StartStatusServer(port string) {
	statusServer = NewStatusServer()

	mux := http.NewServeMux()
	mux.HandleFunc("/status", statusHandler)
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/progress", progressHandler)

	server := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	go func() {
		log.Printf("Status server starting on port %s", port)
		log.Printf("Available endpoints:")
		log.Printf("  http://localhost:%s/status   - Full status information", port)
		log.Printf("  http://localhost:%s/progress - Progress summary", port)
		log.Printf("  http://localhost:%s/health   - Health check", port)

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Status server error: %v", err)
		}
	}()
}
