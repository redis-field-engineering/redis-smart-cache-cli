package cmd

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/cobra"
	"os"
	"smart-cache-cli/mainMenu"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "redis-smartcache-cli",
	Short: "A brief description of your application",
	Long: `A longer description that spans multiple lines and likely contains
examples and usage of using your application. For example:
Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		rdb := redis.NewClient(&redis.Options{
			Addr:     fmt.Sprintf("%s:%s", HostName, Port),
			Password: Password,
			Username: User,
			DB:       0,
		})
		p := tea.NewProgram(mainMenu.InitialModel(rdb))
		if res, err := p.Run(); err != nil {
			fmt.Printf("Alas, there's been an error: %v", err)
			os.Exit(1)
		} else {
			fmt.Println(res.(mainMenu.Model).Choice)
		}
	},
	// Uncomment the following line if your bare application
	// has an action associated with it:
	// Run: func(cmd *cobra.Command, args []string) { },
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
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

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.redis-smartcache-cli.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	rootCmd.PersistentFlags().StringVarP(&HostName, "host", "n", "localhost", "host to connect to Redis on")
	rootCmd.PersistentFlags().StringVarP(&Port, "port", "p", "6379", "the port to connect to Redis on")
	rootCmd.PersistentFlags().StringVarP(&Password, "password", "a", "", "Password for Redis")
	rootCmd.PersistentFlags().StringVarP(&Password, "user", "u", "default", "User to authenticate to Redis with - defaults to 'default'")
}
