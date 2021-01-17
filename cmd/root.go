package cmd

import (
	"errors"
	"net/url"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/ssut/wwwsync/www"
)

const (
	DefaultIndexFetchWorkerCount = 32
	DefaultDownloadWorkerCount   = 8
	DefaultTargetDirectory       = "out"
)

var (
	indexFetchWorkerCount int
	downloadWorkerCount   int
	targetDirectory       string

	rootCmd = &cobra.Command{
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) < 1 {
				return errors.New("Must pass a URL")
			}

			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			targetURL := args[1]
			baseURL, err := url.Parse(targetURL)
			if err != nil {
				panic(err)
			}

			options := www.ClientOptions{
				BaseURL:               *baseURL,
				IndexFetchWorkerCount: indexFetchWorkerCount,
				DownloadWorkerCount:   downloadWorkerCount,
				TargetDirectory:       targetDirectory,
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

	rootCmd.Flags().IntVarP(&indexFetchWorkerCount, "index-workers", "i", indexFetchWorkerCount, "Index fetch worker count")
	rootCmd.Flags().IntVarP(&downloadWorkerCount, "download-workers", "d", downloadWorkerCount, "Download worker count")
	rootCmd.Flags().StringVarP(&targetDirectory, "target-directory", "o", DefaultTargetDirectory, "Target output directory")
}
