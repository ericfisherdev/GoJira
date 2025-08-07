package jira

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// ConnectionPool manages a pool of Jira API clients for optimal performance
type ConnectionPool struct {
	clients     chan *Client
	factory     ClientFactory
	config      PoolConfig
	stats       PoolStats
	mu          sync.RWMutex
	stopCh      chan struct{}
	healthCheck *time.Ticker
	closed      bool
}

// ClientFactory creates new client instances
type ClientFactory func() (*Client, error)

// PoolConfig configures connection pool behavior
type PoolConfig struct {
	MaxSize              int           `json:"maxSize"`              // Maximum number of connections
	MinSize              int           `json:"minSize"`              // Minimum number of connections to maintain
	MaxIdleTime          time.Duration `json:"maxIdleTime"`          // Maximum time a connection can be idle
	AcquisitionTimeout   time.Duration `json:"acquisitionTimeout"`   // Maximum time to wait for a connection
	HealthCheckInterval  time.Duration `json:"healthCheckInterval"`  // How often to check connection health
	MaxConnectionAge     time.Duration `json:"maxConnectionAge"`     // Maximum age of a connection before recreation
	ValidationQuery      string        `json:"validationQuery"`      // Query to validate connections
	EnableHealthCheck    bool          `json:"enableHealthCheck"`    // Whether to enable health checking
	EnableMetrics        bool          `json:"enableMetrics"`        // Whether to collect metrics
}

// PoolStats tracks connection pool performance
type PoolStats struct {
	TotalCreated     int64         `json:"totalCreated"`
	TotalDestroyed   int64         `json:"totalDestroyed"`
	ActiveCount      int           `json:"activeCount"`
	IdleCount        int           `json:"idleCount"`
	AcquisitionCount int64         `json:"acquisitionCount"`
	AcquisitionTime  time.Duration `json:"acquisitionTime"`
	HealthCheckCount int64         `json:"healthCheckCount"`
	FailedChecks     int64         `json:"failedChecks"`
	MaxWaitTime      time.Duration `json:"maxWaitTime"`
	CreationFailures int64         `json:"creationFailures"`
}

// PooledClient wraps a client with pool management metadata
type PooledClient struct {
	*Client
	createdAt    time.Time
	lastUsed     time.Time
	usageCount   int64
	isValid      bool
	pool         *ConnectionPool
}

// NewConnectionPool creates a new connection pool
func NewConnectionPool(factory ClientFactory, config PoolConfig) (*ConnectionPool, error) {
	if config.MaxSize <= 0 {
		config.MaxSize = 10
	}
	if config.MinSize < 0 {
		config.MinSize = 0
	}
	if config.MinSize > config.MaxSize {
		config.MinSize = config.MaxSize
	}
	if config.MaxIdleTime == 0 {
		config.MaxIdleTime = 30 * time.Minute
	}
	if config.AcquisitionTimeout == 0 {
		config.AcquisitionTimeout = 30 * time.Second
	}
	if config.HealthCheckInterval == 0 {
		config.HealthCheckInterval = 5 * time.Minute
	}
	if config.MaxConnectionAge == 0 {
		config.MaxConnectionAge = time.Hour
	}

	pool := &ConnectionPool{
		clients: make(chan *Client, config.MaxSize),
		factory: factory,
		config:  config,
		stopCh:  make(chan struct{}),
		stats:   PoolStats{},
	}

	// Pre-populate pool with minimum connections
	for i := 0; i < config.MinSize; i++ {
		client, err := pool.createClient()
		if err != nil {
			log.Error().Err(err).Msg("Failed to create initial pool connection")
			continue
		}
		
		select {
		case pool.clients <- client:
			pool.updateStats(func(s *PoolStats) { s.IdleCount++ })
		default:
			// Channel full, destroy client
			pool.destroyClient(client)
		}
	}

	// Start health check routine if enabled
	if config.EnableHealthCheck {
		pool.healthCheck = time.NewTicker(config.HealthCheckInterval)
		go pool.healthCheckLoop()
	}

	log.Info().
		Int("maxSize", config.MaxSize).
		Int("minSize", config.MinSize).
		Int("initialConnections", config.MinSize).
		Msg("Connection pool initialized")

	return pool, nil
}

// Get acquires a connection from the pool
func (p *ConnectionPool) Get(ctx context.Context) (*PooledClient, error) {
	if p.closed {
		return nil, fmt.Errorf("connection pool is closed")
	}

	startTime := time.Now()
	defer func() {
		acquisitionTime := time.Since(startTime)
		p.updateStats(func(s *PoolStats) {
			s.AcquisitionCount++
			s.AcquisitionTime = acquisitionTime
			if acquisitionTime > s.MaxWaitTime {
				s.MaxWaitTime = acquisitionTime
			}
		})
	}()

	// Set timeout for acquisition
	ctx, cancel := context.WithTimeout(ctx, p.config.AcquisitionTimeout)
	defer cancel()

	select {
	case client := <-p.clients:
		// Got existing client from pool
		p.updateStats(func(s *PoolStats) {
			s.IdleCount--
			s.ActiveCount++
		})

		pooledClient := &PooledClient{
			Client:     client,
			createdAt:  time.Now(), // This would be set properly during creation
			lastUsed:   time.Now(),
			usageCount: 0,
			isValid:    true,
			pool:       p,
		}

		// Validate client if needed
		if p.config.EnableHealthCheck && !p.validateClient(client) {
			log.Debug().Msg("Client failed validation, creating new one")
			p.destroyClient(client)
			return p.createNewPooledClient()
		}

		return pooledClient, nil

	case <-ctx.Done():
		return nil, fmt.Errorf("timeout acquiring connection from pool: %v", ctx.Err())

	default:
		// No idle connections, try to create new one
		return p.createNewPooledClient()
	}
}

// Return returns a connection to the pool
func (p *ConnectionPool) Return(client *PooledClient) error {
	if p.closed {
		p.destroyClient(client.Client)
		return nil
	}

	p.updateStats(func(s *PoolStats) {
		s.ActiveCount--
	})

	// Check if client should be destroyed
	if !client.isValid || 
	   time.Since(client.createdAt) > p.config.MaxConnectionAge ||
	   len(p.clients) >= p.config.MaxSize {
		p.destroyClient(client.Client)
		return nil
	}

	// Return to pool
	client.lastUsed = time.Now()
	select {
	case p.clients <- client.Client:
		p.updateStats(func(s *PoolStats) { s.IdleCount++ })
		return nil
	default:
		// Pool full, destroy client
		p.destroyClient(client.Client)
		return nil
	}
}

// Close closes the connection pool and all connections
func (p *ConnectionPool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil
	}

	log.Info().Msg("Closing connection pool")
	p.closed = true
	close(p.stopCh)

	if p.healthCheck != nil {
		p.healthCheck.Stop()
	}

	// Close all connections in the pool
	close(p.clients)
	for client := range p.clients {
		p.destroyClient(client)
	}

	log.Info().
		Int64("totalCreated", p.stats.TotalCreated).
		Int64("totalDestroyed", p.stats.TotalDestroyed).
		Msg("Connection pool closed")

	return nil
}

// GetStats returns current pool statistics
func (p *ConnectionPool) GetStats() PoolStats {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.stats
}

// GetDetailedStats returns comprehensive pool statistics
func (p *ConnectionPool) GetDetailedStats() map[string]interface{} {
	stats := p.GetStats()
	
	utilizationRate := float64(0)
	if p.config.MaxSize > 0 {
		utilizationRate = float64(stats.ActiveCount) / float64(p.config.MaxSize) * 100
	}

	healthRate := float64(100)
	if stats.HealthCheckCount > 0 {
		healthRate = float64(stats.HealthCheckCount-stats.FailedChecks) / float64(stats.HealthCheckCount) * 100
	}

	return map[string]interface{}{
		"config": p.config,
		"stats":  stats,
		"health": map[string]interface{}{
			"utilizationRate": utilizationRate,
			"healthRate":      healthRate,
			"poolFull":        len(p.clients) >= p.config.MaxSize,
			"poolEmpty":       len(p.clients) == 0,
		},
	}
}

// Resize changes the pool size limits
func (p *ConnectionPool) Resize(newMaxSize, newMinSize int) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if newMaxSize <= 0 {
		return fmt.Errorf("max size must be positive")
	}
	if newMinSize < 0 {
		return fmt.Errorf("min size cannot be negative")
	}
	if newMinSize > newMaxSize {
		return fmt.Errorf("min size cannot exceed max size")
	}

	oldMaxSize := p.config.MaxSize
	oldMinSize := p.config.MinSize
	
	p.config.MaxSize = newMaxSize
	p.config.MinSize = newMinSize

	// If decreasing max size, remove excess connections
	if newMaxSize < oldMaxSize {
		excess := len(p.clients) - newMaxSize
		for i := 0; i < excess; i++ {
			select {
			case client := <-p.clients:
				p.destroyClient(client)
				p.updateStats(func(s *PoolStats) { s.IdleCount-- })
			default:
				break
			}
		}
	}

	// If increasing min size, add more connections
	if newMinSize > oldMinSize {
		needed := newMinSize - len(p.clients)
		for i := 0; i < needed; i++ {
			client, err := p.createClient()
			if err != nil {
				log.Error().Err(err).Msg("Failed to create connection during resize")
				continue
			}
			
			select {
			case p.clients <- client:
				p.updateStats(func(s *PoolStats) { s.IdleCount++ })
			default:
				p.destroyClient(client)
				break
			}
		}
	}

	log.Info().
		Int("oldMaxSize", oldMaxSize).
		Int("newMaxSize", newMaxSize).
		Int("oldMinSize", oldMinSize).
		Int("newMinSize", newMinSize).
		Msg("Connection pool resized")

	return nil
}

// Helper methods

func (p *ConnectionPool) createClient() (*Client, error) {
	client, err := p.factory()
	if err != nil {
		p.updateStats(func(s *PoolStats) { s.CreationFailures++ })
		return nil, fmt.Errorf("failed to create client: %v", err)
	}

	p.updateStats(func(s *PoolStats) { s.TotalCreated++ })
	
	log.Debug().Msg("Created new connection pool client")
	return client, nil
}

func (p *ConnectionPool) createNewPooledClient() (*PooledClient, error) {
	client, err := p.createClient()
	if err != nil {
		return nil, err
	}

	p.updateStats(func(s *PoolStats) { s.ActiveCount++ })

	return &PooledClient{
		Client:     client,
		createdAt:  time.Now(),
		lastUsed:   time.Now(),
		usageCount: 0,
		isValid:    true,
		pool:       p,
	}, nil
}

func (p *ConnectionPool) destroyClient(client *Client) {
	if client != nil {
		// Perform any cleanup on the client
		// For HTTP clients, this might involve closing connections
		p.updateStats(func(s *PoolStats) { s.TotalDestroyed++ })
		log.Debug().Msg("Destroyed connection pool client")
	}
}

func (p *ConnectionPool) validateClient(client *Client) bool {
	if !p.config.EnableHealthCheck {
		return true
	}

	p.updateStats(func(s *PoolStats) { s.HealthCheckCount++ })

	// Simple health check - attempt to get server info
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.HealthCheck(ctx); err != nil {
		log.Debug().Err(err).Msg("Client health check failed")
		p.updateStats(func(s *PoolStats) { s.FailedChecks++ })
		return false
	}

	return true
}

func (p *ConnectionPool) updateStats(updateFn func(*PoolStats)) {
	if !p.config.EnableMetrics {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	updateFn(&p.stats)
}

func (p *ConnectionPool) healthCheckLoop() {
	for {
		select {
		case <-p.healthCheck.C:
			p.performHealthChecks()
		case <-p.stopCh:
			return
		}
	}
}

func (p *ConnectionPool) performHealthChecks() {
	// Create a temporary slice to hold clients for health checking
	var clientsToCheck []*Client
	
	// Drain some clients from the pool for health checking
	checkCount := len(p.clients) / 2 // Check half the idle connections
	if checkCount > 5 {
		checkCount = 5 // Limit to 5 at a time
	}

	for i := 0; i < checkCount; i++ {
		select {
		case client := <-p.clients:
			clientsToCheck = append(clientsToCheck, client)
			p.updateStats(func(s *PoolStats) { s.IdleCount-- })
		default:
			break
		}
	}

	// Check each client and return valid ones to the pool
	for _, client := range clientsToCheck {
		if p.validateClient(client) {
			select {
			case p.clients <- client:
				p.updateStats(func(s *PoolStats) { s.IdleCount++ })
			default:
				// Pool full, destroy client
				p.destroyClient(client)
			}
		} else {
			// Invalid client, destroy and create new one if below min size
			p.destroyClient(client)
			
			if len(p.clients) < p.config.MinSize {
				if newClient, err := p.createClient(); err == nil {
					select {
					case p.clients <- newClient:
						p.updateStats(func(s *PoolStats) { s.IdleCount++ })
					default:
						p.destroyClient(newClient)
					}
				}
			}
		}
	}

	log.Debug().
		Int("checkedClients", len(clientsToCheck)).
		Int("poolSize", len(p.clients)).
		Msg("Completed health check cycle")
}

// PooledClient methods

// Use increments the usage count and updates last used time
func (pc *PooledClient) Use() {
	pc.usageCount++
	pc.lastUsed = time.Now()
}

// IsStale checks if the client is stale and should be replaced
func (pc *PooledClient) IsStale() bool {
	return time.Since(pc.lastUsed) > pc.pool.config.MaxIdleTime ||
		   time.Since(pc.createdAt) > pc.pool.config.MaxConnectionAge
}

// Invalidate marks the client as invalid
func (pc *PooledClient) Invalidate() {
	pc.isValid = false
}

// Close returns the client to the pool
func (pc *PooledClient) Close() error {
	return pc.pool.Return(pc)
}

// Configuration presets

// DefaultPoolConfig returns a sensible default pool configuration
func DefaultPoolConfig() PoolConfig {
	return PoolConfig{
		MaxSize:             10,
		MinSize:             2,
		MaxIdleTime:         30 * time.Minute,
		AcquisitionTimeout:  30 * time.Second,
		HealthCheckInterval: 5 * time.Minute,
		MaxConnectionAge:    time.Hour,
		EnableHealthCheck:   true,
		EnableMetrics:       true,
	}
}

// HighVolumePoolConfig returns configuration optimized for high volume
func HighVolumePoolConfig() PoolConfig {
	return PoolConfig{
		MaxSize:             25,
		MinSize:             5,
		MaxIdleTime:         15 * time.Minute,
		AcquisitionTimeout:  10 * time.Second,
		HealthCheckInterval: 2 * time.Minute,
		MaxConnectionAge:    30 * time.Minute,
		EnableHealthCheck:   true,
		EnableMetrics:       true,
	}
}

// LowResourcePoolConfig returns configuration optimized for resource constraints
func LowResourcePoolConfig() PoolConfig {
	return PoolConfig{
		MaxSize:             5,
		MinSize:             1,
		MaxIdleTime:         45 * time.Minute,
		AcquisitionTimeout:  60 * time.Second,
		HealthCheckInterval: 10 * time.Minute,
		MaxConnectionAge:    2 * time.Hour,
		EnableHealthCheck:   false,
		EnableMetrics:       false,
	}
}