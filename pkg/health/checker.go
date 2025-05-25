package health

import (
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/go-logr/logr"
	"github.com/yourusername/k8s-acl-operator/pkg/metrics"
)

// Checker tracks operator health state
type Checker struct {
	ready         int32
	healthy       int32
	lastReconcile int64
	logger        logr.Logger
}

// NewChecker creates a health checker
func NewChecker(logger logr.Logger) *Checker {
	return &Checker{
		healthy:       1, // Start healthy
		ready:         0, // Not ready until initialized
		lastReconcile: time.Now().Unix(),
		logger:        logger,
	}
}

// SetReady marks operator as ready/not ready
func (c *Checker) SetReady(ready bool) {
	if ready {
		atomic.StoreInt32(&c.ready, 1)
		c.logger.Info("Operator marked as ready")
	} else {
		atomic.StoreInt32(&c.ready, 0)
		c.logger.Info("Operator marked as not ready")
	}
}

// SetHealthy marks operator as healthy/unhealthy
func (c *Checker) SetHealthy(healthy bool) {
	if healthy {
		atomic.StoreInt32(&c.healthy, 1)
		atomic.StoreInt64(&c.lastReconcile, time.Now().Unix())
	} else {
		atomic.StoreInt32(&c.healthy, 0)
		c.logger.Info("Operator marked as unhealthy")
	}
	metrics.SetOperatorHealth("health_checker", healthy)
}

// RecordReconcile updates last successful reconcile time
func (c *Checker) RecordReconcile() {
	atomic.StoreInt64(&c.lastReconcile, time.Now().Unix())
	atomic.StoreInt32(&c.healthy, 1)
	metrics.SetOperatorHealth("health_checker", true)
}

// IsReady returns readiness state
func (c *Checker) IsReady() bool {
	return atomic.LoadInt32(&c.ready) == 1
}

// IsHealthy returns health state
func (c *Checker) IsHealthy() bool {
	if atomic.LoadInt32(&c.healthy) == 0 {
		return false
	}

	// Consider unhealthy if no reconcile activity for 5 minutes
	lastReconcile := atomic.LoadInt64(&c.lastReconcile)
	if time.Since(time.Unix(lastReconcile, 0)) > 5*time.Minute {
		c.logger.Info("No reconcile activity detected, marking unhealthy")
		return false
	}

	return true
}

// LivenessCheck implements healthz check
func (c *Checker) LivenessCheck(req *http.Request) error {
	if !c.IsHealthy() {
		return fmt.Errorf("operator unhealthy")
	}
	return nil
}

// ReadinessCheck implements readyz check
func (c *Checker) ReadinessCheck(req *http.Request) error {
	if !c.IsReady() || !c.IsHealthy() {
		return fmt.Errorf("operator not ready")
	}
	return nil
}
