package cmd

import (
	"fmt"
	"os"
	"smart-cache-cli/RedisCommon"
	"smart-cache-cli/mainMenu"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/cobra"
)

const (
	version = "0.0.10"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "redis-smartcache-cli",
	Short: "CLI for interacting with and configuring Redis Smart Cache",
	Long: `CLI for interacting with and configuring Redis Smart Cache. View Smart Cache 
query anlytics, create query caching rules, and reset Smart Cache configuration.`,
	Run: func(cmd *cobra.Command, args []string) {
		if versionCheck {
			fmt.Printf("Redis Smart Cache CLI version v%s\n", version)
			os.Exit(0)
		}
		rdb := redis.NewClient(&redis.Options{
			Addr:     fmt.Sprintf("%s:%s", HostName, Port),
			Password: Password,
			Username: User,
			DB:       0,
			Protocol: 2,
		})

		err := RedisCommon.Ping(rdb)

		if err != nil {
			fmt.Printf("Error connecting to Redis: \"%s\".\n", err.Error())
			os.Exit(1)
		}

		err = RedisCommon.CheckSmartCacheIndex(rdb, ApplicationName)

		if err != nil {
			fmt.Printf("Error checking Redis Smart Cache configuration: %s\n", err)
			os.Exit(1)
		}

		p := tea.NewProgram(mainMenu.InitialModel(rdb, ApplicationName, fmt.Sprintf("%s:%s", HostName, Port)))
		if res, err := p.Run(); err != nil {
			fmt.Printf("Smart Cache CLI error: %v", err)
			os.Exit(1)
		} else {
			fmt.Println(res.(mainMenu.Model).Choice)
		}
	},
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

var HostName string
var Port string
var User string
var Password string
var ApplicationName string
var versionCheck bool
var (
	sortby        string
	sortDirection string
)

func init() {
	rootCmd.PersistentFlags().StringVarP(&HostName, "host", "n", "localhost", "Redis host")
	rootCmd.PersistentFlags().StringVarP(&Port, "port", "p", "6379", "Redis port")
	rootCmd.PersistentFlags().StringVarP(&Password, "password", "a", "", "Redis password")
	rootCmd.PersistentFlags().StringVarP(&User, "user", "u", "default", "Redis user")
	rootCmd.PersistentFlags().StringVarP(&ApplicationName, "application", "s", "smartcache", "Application namespace")
	rootCmd.Flags().BoolVarP(&versionCheck, "version", "v", false, "Smart Cache CLI version")
}
