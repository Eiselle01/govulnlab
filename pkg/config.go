package pkg

import (
	"fmt"

	"github.com/spf13/viper"
)

// Config 全局配置结构体，字段和yaml一一对应
type Config struct {
	DB struct {
		Driver string `mapstructure:"driver"`
		Dsn    string `mapstructure:"dsn"`
	} `mapstructure:"db"`

	Redis struct {
		Addr     string `mapstructure:"addr"`
		Password string `mapstructure:"password"`
		DB       int    `mapstructure:"db"`
	} `mapstructure:"redis"`

	Session struct {
		TTLHour int `mapstructure:"ttl_hour"`
	} `mapstructure:"session"`
}

// GlobalConfig 全局配置实例，项目各处直接使用 pkg.GlobalConfig
var GlobalConfig *Config

// InitConfig 加载配置文件
func InitConfig() error {
	viper.SetConfigName("config") //配置文件名 config.yaml
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".") // 在项目根目录寻找配置文件

	err := viper.ReadInConfig()
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	var cfg Config
	// 将yaml配置反序列化到结构体
	err = viper.Unmarshal(&cfg)
	if err != nil {
		return fmt.Errorf("解析配置失败: %w", err)
	}

	GlobalConfig = &cfg
	return nil
}