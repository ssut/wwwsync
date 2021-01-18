package www

import (
	logrus "github.com/sirupsen/logrus"
	prefixed "github.com/x-cray/logrus-prefixed-formatter"
)

var logger = logrus.New()

func initLogger() {
	formatter := new(prefixed.TextFormatter)
	logger.Formatter = formatter
}

func SetLogLevel(level logrus.Level) {
	logger.Level = level
}
