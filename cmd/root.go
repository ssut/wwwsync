package cmd

import (
	"net/url"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/ssut/wwwsync/www"
)

var (
	rootCmd = &cobra.Command{
		Run: func(cmd *cobra.Command, args []string) {
			baseURL, _ := url.Parse("https://dl-cdn.alpinelinux.org")
			options := www.ClientOptions{
				BaseURL:         *baseURL,
				WorkerCount:     8,
				TargetDirectory: "./out",
			}
			client := www.NewClient(options)
			client.Start()
		},
	}
)

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	viper.SetDefault("license", "MIT")
}
