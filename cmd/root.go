package cmd

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/cobra"
	"os"
	"smart-cache-cli/mainMenu"
)

const (
	version = "0.0.6"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "redis-smartcache-cli",
	Short: "A CLI for interacting with the Redis Smart Cache",
	Long: `A CLI for interacting with the Redis Smart Cache, use this to view the results of 
smart cache profiling and ot create rules that smartcache will use to cache your queries.`,
	Run: func(cmd *cobra.Command, args []string) {
		if versionCheck {
			fmt.Printf("Redis Smart Cache CLI Version: v%s\n", version)
			os.Exit(0)
		}
		rdb := redis.NewClient(&redis.Options{
			Addr:     fmt.Sprintf("%s:%s", HostName, Port),
			Password: Password,
			Username: User,
			DB:       0,
		})
		p := tea.NewProgram(mainMenu.InitialModel(rdb, ApplicationName))
		if res, err := p.Run(); err != nil {
			fmt.Printf("Alas, there's been an error: %v", err)
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
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	rootCmd.PersistentFlags().StringVarP(&HostName, "host", "n", "localhost", "host to connect to Redis on")
	rootCmd.PersistentFlags().StringVarP(&Port, "port", "p", "6379", "the port to connect to Redis on")
	rootCmd.PersistentFlags().StringVarP(&Password, "password", "a", "", "Password for Redis")
	rootCmd.PersistentFlags().StringVarP(&Password, "user", "u", "default", "User to authenticate to Redis with - defaults to 'default'")
	rootCmd.PersistentFlags().StringVarP(&ApplicationName, "application", "s", "smartcache", "The application namespace to use defaults to 'smartcache'")
	rootCmd.Flags().BoolVarP(&versionCheck, "version", "v", false, "Print version.")
}
