// Package serverconfig 服务器配置管理
package serverconfig

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// ServerConfig 服务器配置结构
type ServerConfig struct {
	LevelName                   string `mapstructure:"level-name" yaml:"level-name"`
	LevelPath                   string `mapstructure:"level-path" yaml:"level-path"`
	LevelType                   string `mapstructure:"level-type" yaml:"level-type"`
	MaxPlayers                  int    `mapstructure:"max-players" yaml:"max-players"`
	MOTD                        string `mapstructure:"motd" yaml:"motd"`
	NetworkCompressionThreshold int    `mapstructure:"network-compression-threshold" yaml:"network-compression-threshold"`
	OnlineMode                  bool   `mapstructure:"online-mode" yaml:"online-mode"`
	QueryPort                   int    `mapstructure:"query.port" yaml:"query.port"`
	RateLimit                   int    `mapstructure:"rate-limit" yaml:"rate-limit"`
	RCONPassword                string `mapstructure:"rcon.password" yaml:"rcon.password"`
	RCONPort                    int    `mapstructure:"rcon.port" yaml:"rcon.port"`
	ServerIP                    string `mapstructure:"server-ip" yaml:"server-ip"`
	ServerPort                  int    `mapstructure:"server-port" yaml:"server-port"`
	WhiteList                   bool   `mapstructure:"white-list" yaml:"white-list"`
	TickMode                    string `mapstructure:"tick-mode" yaml:"tick-mode"`
	TerrainSeed                 int64  `mapstructure:"terrain-seed" yaml:"terrain-seed"`
}

// DefaultConfig 默认配置
var DefaultConfig = &ServerConfig{
	LevelName:                   "world",
	LevelType:                   "minecraft:normal",
	MaxPlayers:                  20,
	MOTD:                        "A Minecraft Server",
	NetworkCompressionThreshold: 256,
	OnlineMode:                  true,
	QueryPort:                   25565,
	RateLimit:                   0,
	RCONPassword:                "",
	RCONPort:                    25575,
	ServerIP:                    "0.0.0.0",
	ServerPort:                  25565,
	WhiteList:                   false,
	TickMode:                    "singlethread",
	TerrainSeed:                 0,
}

// LoadConfig 从配置文件加载服务器配置
func LoadConfig(configPath string) (*ServerConfig, error) {
	viper.SetConfigFile(configPath)
	viper.SetConfigType("yaml")

	err := viper.ReadInConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %v", err)
	}

	if shouldGenerateTerrainSeed() {
		seed := generateTerrainSeed()
		viper.Set("terrain-seed", seed)
		if err := viper.WriteConfig(); err != nil {
			return nil, fmt.Errorf("failed to persist generated terrain-seed: %v", err)
		}
	}

	config := &ServerConfig{}
	err = viper.Unmarshal(config)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %v", err)
	}

	return config, nil
}

// SaveConfig 保存配置到文件
func SaveConfig(config *ServerConfig, configPath string) error {
	viper.Set("level-name", config.LevelName)
	viper.Set("level-type", config.LevelType)
	viper.Set("max-players", config.MaxPlayers)
	viper.Set("motd", config.MOTD)
	viper.Set("network-compression-threshold", config.NetworkCompressionThreshold)
	viper.Set("online-mode", config.OnlineMode)
	viper.Set("query.port", config.QueryPort)
	viper.Set("rate-limit", config.RateLimit)
	viper.Set("rcon.password", config.RCONPassword)
	viper.Set("rcon.port", config.RCONPort)
	viper.Set("server-ip", config.ServerIP)
	viper.Set("server-port", config.ServerPort)
	viper.Set("white-list", config.WhiteList)
	viper.Set("tick-mode", config.TickMode)
	viper.Set("terrain-seed", config.TerrainSeed)

	return viper.WriteConfigAs(configPath)
}

func shouldGenerateTerrainSeed() bool {
	if !viper.IsSet("terrain-seed") {
		return true
	}

	raw := viper.Get("terrain-seed")
	if raw == nil {
		return true
	}

	if str, ok := raw.(string); ok && strings.TrimSpace(str) == "" {
		return true
	}

	return false
}

func generateTerrainSeed() int64 {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		seed := time.Now().UnixNano() & 0x7fffffffffffffff
		if seed == 0 {
			return 1
		}
		return seed
	}

	seed := int64(binary.LittleEndian.Uint64(b[:]) & 0x7fffffffffffffff)
	if seed == 0 {
		return 1
	}
	return seed
}

// CreateDefaultConfig 创建默认配置文件
func CreateDefaultConfig(configPath string) error {
	// 确保目录存在
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %v", err)
	}

	file, err := os.Create(configPath)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	// 使用标准的 YAML 格式
	content := `# Minecraft Server Config
level-name: world
level-type: minecraft:normal
max-players: 20
motd: A Minecraft Server
network-compression-threshold: 256
online-mode: true
query.port: 25565
rate-limit: 0
rcon.password: ""
rcon.port: 25575
server-ip: 0.0.0.0
server-port: 25565
white-list: false

# Tick Mode: singlethread | multithread
tick-mode: singlethread

# Terrain seed: leave empty to auto-generate on first startup
terrain-seed:
`
	_, err = file.WriteString(content)
	return err
}

// Validate 验证配置的有效性
func (c *ServerConfig) Validate() error {
	if c.ServerPort < 1 || c.ServerPort > 65535 {
		return fmt.Errorf("invalid server port: %d", c.ServerPort)
	}

	if c.QueryPort < 1 || c.QueryPort > 65535 {
		return fmt.Errorf("invalid query port: %d", c.QueryPort)
	}

	if c.RCONPort < 1 || c.RCONPort > 65535 {
		return fmt.Errorf("invalid RCON port: %d", c.RCONPort)
	}

	if c.MaxPlayers <= 0 {
		return fmt.Errorf("invalid max players: %d", c.MaxPlayers)
	}

	if c.NetworkCompressionThreshold < 0 {
		return fmt.Errorf("invalid network compression threshold: %d", c.NetworkCompressionThreshold)
	}

	if c.TickMode != "singlethread" && c.TickMode != "multithread" {
		return fmt.Errorf("invalid tick-mode: %s (must be 'singlethread' or 'multithread')", c.TickMode)
	}

	return nil
}

// GetServerAddress 获取服务器地址
func (c *ServerConfig) GetServerAddress() string {
	return fmt.Sprintf("%s:%d", c.ServerIP, c.ServerPort)
}

// GetQueryAddress 获取查询地址
func (c *ServerConfig) GetQueryAddress() string {
	return fmt.Sprintf("%s:%d", c.ServerIP, c.QueryPort)
}

// GetRCONAddress 获取 RCON 地址
func (c *ServerConfig) GetRCONAddress() string {
	return fmt.Sprintf("%s:%d", c.ServerIP, c.RCONPort)
}
