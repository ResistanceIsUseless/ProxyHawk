package server

import (
	"fmt"
	"io/ioutil"
	"time"
	
	"gopkg.in/yaml.v3"
)

// ServerConfig represents the complete server configuration
type ServerConfig struct {
	Server   ServerSettings            `yaml:"server"`
	Regions  map[string]*RegionConfig  `yaml:"regions"`
	Selection SelectionSettings        `yaml:"selection"`
	RoundRobin RoundRobinSettings       `yaml:"round_robin_detection"`
	HealthCheck HealthCheckSettings     `yaml:"health_check"`
	Cache    CacheSettings             `yaml:"cache"`
	Logging  LoggingSettings           `yaml:"logging"`
}

// ServerSettings holds basic server configuration
type ServerSettings struct {
	Proxy ProxySettings `yaml:"proxy"`
	API   APISettings   `yaml:"api"`
}

// ProxySettings holds proxy server configuration
type ProxySettings struct {
	SOCKS5 SOCKS5Settings `yaml:"socks5"`
	HTTP   HTTPSettings   `yaml:"http"`
}

// SOCKS5Settings holds SOCKS5 proxy configuration
type SOCKS5Settings struct {
	Enabled bool   `yaml:"enabled"`
	Address string `yaml:"address"`
	Auth    struct {
		Enabled  bool   `yaml:"enabled"`
		Username string `yaml:"username"`
		Password string `yaml:"password"`
	} `yaml:"auth"`
}

// HTTPSettings holds HTTP proxy configuration
type HTTPSettings struct {
	Enabled     bool   `yaml:"enabled"`
	Address     string `yaml:"address"`
	Transparent bool   `yaml:"transparent"`
}

// APISettings holds API/WebSocket configuration
type APISettings struct {
	Enabled bool        `yaml:"enabled"`
	Address string      `yaml:"address"`
	CORS    CORSSettings `yaml:"cors"`
	WebSocket WSSettings `yaml:"websocket"`
}

// CORSSettings holds CORS configuration
type CORSSettings struct {
	Enabled bool     `yaml:"enabled"`
	Origins []string `yaml:"origins"`
}

// WSSettings holds WebSocket configuration
type WSSettings struct {
	Enabled        bool          `yaml:"enabled"`
	Path           string        `yaml:"path"`
	MaxConnections int           `yaml:"max_connections"`
	PingInterval   time.Duration `yaml:"ping_interval"`
}

// SelectionSettings holds proxy selection configuration
type SelectionSettings struct {
	Strategy string       `yaml:"strategy"`
	Smart    SmartSettings `yaml:"smart"`
	Sticky   StickySettings `yaml:"sticky"`
}

// SmartSettings holds smart selection configuration
type SmartSettings struct {
	CDNDetection bool `yaml:"cdn_detection"`
	GeoIPLookup  bool `yaml:"geoip_lookup"`
}

// StickySettings holds sticky session configuration
type StickySettings struct {
	SessionDuration time.Duration `yaml:"session_duration"`
	ByDomain        bool          `yaml:"by_domain"`
}

// RoundRobinSettings holds round-robin detection configuration
type RoundRobinSettings struct {
	Enabled             bool          `yaml:"enabled"`
	MinSamples          int           `yaml:"min_samples"`
	SampleInterval      time.Duration `yaml:"sample_interval"`
	ConfidenceThreshold float64       `yaml:"confidence_threshold"`
	Cache               struct {
		Enabled    bool          `yaml:"enabled"`
		TTL        time.Duration `yaml:"ttl"`
		MaxEntries int           `yaml:"max_entries"`
	} `yaml:"cache"`
}

// HealthCheckSettings holds health check configuration
type HealthCheckSettings struct {
	Enabled          bool          `yaml:"enabled"`
	Interval         time.Duration `yaml:"interval"`
	Timeout          time.Duration `yaml:"timeout"`
	FailureThreshold int           `yaml:"failure_threshold"`
	SuccessThreshold int           `yaml:"success_threshold"`
}

// CacheSettings holds cache configuration
type CacheSettings struct {
	Enabled    bool          `yaml:"enabled"`
	TTL        time.Duration `yaml:"ttl"`
	MaxEntries int           `yaml:"max_entries"`
}

// LoggingSettings holds logging configuration
type LoggingSettings struct {
	Level   string        `yaml:"level"`
	Format  string        `yaml:"format"`
	Outputs []LogOutput   `yaml:"outputs"`
}

// LogOutput represents a logging output destination
type LogOutput struct {
	Type string `yaml:"type"`
	Path string `yaml:"path,omitempty"`
}

// LoadConfig loads configuration from a YAML file
func LoadConfig(filename string) (*Config, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	
	var serverConfig ServerConfig
	if err := yaml.Unmarshal(data, &serverConfig); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}
	
	// Convert to internal config format
	config := &Config{
		Mode:       ModeDual, // Default mode
		SOCKS5Addr: serverConfig.Server.Proxy.SOCKS5.Address,
		HTTPAddr:   serverConfig.Server.Proxy.HTTP.Address,
		APIAddr:    serverConfig.Server.API.Address,
		Regions:    serverConfig.Regions,
		
		SelectionStrategy: SelectionStrategy(serverConfig.Selection.Strategy),
		
		RoundRobinDetection: RoundRobinConfig{
			Enabled:             serverConfig.RoundRobin.Enabled,
			MinSamples:          serverConfig.RoundRobin.MinSamples,
			SampleInterval:      serverConfig.RoundRobin.SampleInterval,
			ConfidenceThreshold: serverConfig.RoundRobin.ConfidenceThreshold,
		},
		
		HealthCheck: HealthCheckConfig{
			Enabled:          serverConfig.HealthCheck.Enabled,
			Interval:         serverConfig.HealthCheck.Interval,
			Timeout:          serverConfig.HealthCheck.Timeout,
			FailureThreshold: serverConfig.HealthCheck.FailureThreshold,
			SuccessThreshold: serverConfig.HealthCheck.SuccessThreshold,
		},
		
		CacheConfig: CacheConfig{
			Enabled:    serverConfig.Cache.Enabled,
			TTL:        serverConfig.Cache.TTL,
			MaxEntries: serverConfig.Cache.MaxEntries,
		},
		
		LogLevel:  serverConfig.Logging.Level,
		LogFormat: serverConfig.Logging.Format,
	}
	
	return config, nil
}

// SaveConfig saves configuration to a YAML file
func SaveConfig(config *Config, filename string) error {
	// Convert from internal config format
	serverConfig := ServerConfig{
		Server: ServerSettings{
			Proxy: ProxySettings{
				SOCKS5: SOCKS5Settings{
					Enabled: true,
					Address: config.SOCKS5Addr,
				},
				HTTP: HTTPSettings{
					Enabled: true,
					Address: config.HTTPAddr,
				},
			},
			API: APISettings{
				Enabled: true,
				Address: config.APIAddr,
				CORS: CORSSettings{
					Enabled: true,
					Origins: []string{"http://localhost:*"},
				},
				WebSocket: WSSettings{
					Enabled:        true,
					Path:           "/ws",
					MaxConnections: 100,
					PingInterval:   30 * time.Second,
				},
			},
		},
		
		Regions: config.Regions,
		
		Selection: SelectionSettings{
			Strategy: string(config.SelectionStrategy),
			Smart: SmartSettings{
				CDNDetection: true,
				GeoIPLookup:  true,
			},
			Sticky: StickySettings{
				SessionDuration: 5 * time.Minute,
				ByDomain:        true,
			},
		},
		
		RoundRobin: RoundRobinSettings{
			Enabled:             config.RoundRobinDetection.Enabled,
			MinSamples:          config.RoundRobinDetection.MinSamples,
			SampleInterval:      config.RoundRobinDetection.SampleInterval,
			ConfidenceThreshold: config.RoundRobinDetection.ConfidenceThreshold,
		},
		
		HealthCheck: HealthCheckSettings{
			Enabled:          config.HealthCheck.Enabled,
			Interval:         config.HealthCheck.Interval,
			Timeout:          config.HealthCheck.Timeout,
			FailureThreshold: config.HealthCheck.FailureThreshold,
			SuccessThreshold: config.HealthCheck.SuccessThreshold,
		},
		
		Cache: CacheSettings{
			Enabled:    config.CacheConfig.Enabled,
			TTL:        config.CacheConfig.TTL,
			MaxEntries: config.CacheConfig.MaxEntries,
		},
		
		Logging: LoggingSettings{
			Level:  config.LogLevel,
			Format: config.LogFormat,
			Outputs: []LogOutput{
				{Type: "stdout"},
			},
		},
	}
	
	data, err := yaml.Marshal(&serverConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	
	if err := ioutil.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}
	
	return nil
}

// ValidateConfig validates the configuration
func ValidateConfig(config *Config) error {
	if config.SOCKS5Addr == "" {
		return fmt.Errorf("SOCKS5 address cannot be empty")
	}
	
	if config.HTTPAddr == "" {
		return fmt.Errorf("HTTP address cannot be empty")
	}
	
	if config.APIAddr == "" {
		return fmt.Errorf("API address cannot be empty")
	}
	
	if len(config.Regions) == 0 {
		return fmt.Errorf("at least one region must be configured")
	}
	
	// Validate each region
	for name, region := range config.Regions {
		if region.Name == "" {
			return fmt.Errorf("region %s must have a name", name)
		}
		
		if len(region.Proxies) == 0 {
			return fmt.Errorf("region %s must have at least one proxy", name)
		}
		
		// Validate each proxy
		for i, proxy := range region.Proxies {
			if proxy.URL == "" {
				return fmt.Errorf("region %s proxy %d must have a URL", name, i)
			}
			
			if proxy.Weight <= 0 {
				return fmt.Errorf("region %s proxy %d must have a positive weight", name, i)
			}
		}
	}
	
	// Validate selection strategy
	switch config.SelectionStrategy {
	case StrategyRoundRobin, StrategyRandom, StrategyWeighted, StrategySmart, StrategySticky:
		// Valid strategies
	default:
		return fmt.Errorf("invalid selection strategy: %s", config.SelectionStrategy)
	}
	
	// Validate health check settings
	if config.HealthCheck.Enabled {
		if config.HealthCheck.Interval <= 0 {
			return fmt.Errorf("health check interval must be positive")
		}
		
		if config.HealthCheck.Timeout <= 0 {
			return fmt.Errorf("health check timeout must be positive")
		}
		
		if config.HealthCheck.FailureThreshold <= 0 {
			return fmt.Errorf("health check failure threshold must be positive")
		}
		
		if config.HealthCheck.SuccessThreshold <= 0 {
			return fmt.Errorf("health check success threshold must be positive")
		}
	}
	
	// Validate cache settings
	if config.CacheConfig.Enabled {
		if config.CacheConfig.TTL <= 0 {
			return fmt.Errorf("cache TTL must be positive")
		}
		
		if config.CacheConfig.MaxEntries <= 0 {
			return fmt.Errorf("cache max entries must be positive")
		}
	}
	
	// Validate round-robin detection settings
	if config.RoundRobinDetection.Enabled {
		if config.RoundRobinDetection.MinSamples <= 1 {
			return fmt.Errorf("round-robin min samples must be greater than 1")
		}
		
		if config.RoundRobinDetection.SampleInterval <= 0 {
			return fmt.Errorf("round-robin sample interval must be positive")
		}
		
		if config.RoundRobinDetection.ConfidenceThreshold <= 0 || config.RoundRobinDetection.ConfidenceThreshold > 1 {
			return fmt.Errorf("round-robin confidence threshold must be between 0 and 1")
		}
	}
	
	return nil
}

// GetDefaultConfigYAML returns a default configuration in YAML format
func GetDefaultConfigYAML() string {
	return `# ProxyHawk Dual-Mode Server Configuration

server:
  # Proxy server settings
  proxy:
    socks5:
      enabled: true
      address: ":1080"
      auth:
        enabled: false
        username: ""
        password: ""
    
    http:
      enabled: true
      address: ":8080"
      transparent: true  # Transparent proxy mode
    
  # WebSocket/API server
  api:
    enabled: true
    address: ":8888"
    cors:
      enabled: true
      origins: ["http://localhost:*"]
    
    websocket:
      enabled: true
      path: "/ws"
      max_connections: 100
      ping_interval: "30s"

# Regional proxy configuration
regions:
  us_west:
    name: "US West Coast"
    proxies:
      - url: "socks5://us-west-1.proxy.example.com:1080"
        weight: 10
        health_check_url: "http://httpbin.org/ip"
      - url: "http://us-west-2.proxy.example.com:8080"
        weight: 5
        
  us_east:
    name: "US East Coast"
    proxies:
      - url: "socks5://us-east-1.proxy.example.com:1080"
        weight: 10
        
  eu_west:
    name: "Western Europe"
    proxies:
      - url: "socks5://eu-1.proxy.example.com:1080"
        weight: 10
        
  asia:
    name: "Asia Pacific"
    proxies:
      - url: "socks5://sg-1.proxy.example.com:1080"
        weight: 8
      - url: "socks5://jp-1.proxy.example.com:1080"
        weight: 7

# Proxy selection strategies
selection:
  strategy: "smart"  # Options: smart, round-robin, random, sticky, weighted
  
  smart:
    # Automatically select region based on target
    cdn_detection: true
    geoip_lookup: true
    
  sticky:
    # Keep same proxy for duration
    session_duration: "5m"
    by_domain: true

# Round-robin detection
round_robin_detection:
  enabled: true
  min_samples: 5
  sample_interval: "2s"
  confidence_threshold: 0.85
  
  # Cache settings
  cache:
    enabled: true
    ttl: "5m"
    max_entries: 10000

# Health checking
health_check:
  enabled: true
  interval: "1m"
  timeout: "10s"
  failure_threshold: 3
  success_threshold: 2

# Global cache settings
cache:
  enabled: true
  ttl: "5m"
  max_entries: 10000

# Logging
logging:
  level: "info"
  format: "json"
  outputs:
    - type: "file"
      path: "/var/log/proxyhawk/server.log"
    - type: "stdout"
`
}