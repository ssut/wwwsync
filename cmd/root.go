package cmd

import (
	"errors"
	"net/url"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/ssut/wwwsync/www"
)

const (
	DefaultIndexFetchWorkerCount = 32
	DefaultDownloadWorkerCount   = 8
	DefaultTargetDirectory       = "out"
	DefaultSkipExisting          = false
)

var (
	indexFetchWorkerCount int    = DefaultIndexFetchWorkerCount
	downloadWorkerCount   int    = DefaultDownloadWorkerCount
	targetDirectory       string = DefaultTargetDirectory
	skipExisting          bool   = DefaultSkipExisting
	verbose                      = false

	rootCmd = &cobra.Command{
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) < 1 {
				return errors.New("Must pass a URL")
			}

			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			if verbose {
				www.SetLogLevel(logrus.DebugLevel)
			}

			targetURL := args[0]
			baseURL, err := url.Parse(targetURL)
			if err != nil {
				panic(err)
			}

			options := www.ClientOptions{
				BaseURL:               *baseURL,
				IndexFetchWorkerCount: indexFetchWorkerCount,
				DownloadWorkerCount:   downloadWorkerCount,
				TargetDirectory:       targetDirectory,
				SkipExisting:          skipExisting,
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

	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "")

	rootCmd.Flags().IntVarP(&indexFetchWorkerCount, "index-workers", "i", indexFetchWorkerCount, "Index fetch worker count")
	rootCmd.Flags().IntVarP(&downloadWorkerCount, "download-workers", "d", downloadWorkerCount, "Download worker count")
	rootCmd.Flags().StringVarP(&targetDirectory, "target-directory", "o", DefaultTargetDirectory, "Target output directory")
	rootCmd.Flags().BoolVarP(&skipExisting, "skip-existing", "s", DefaultSkipExisting, "Skip existing files")
}
