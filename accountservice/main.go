package main

import (
	"bytes"
	"flag"
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/callistaenterprise/goblog/accountservice/dbclient"
	"github.com/callistaenterprise/goblog/accountservice/service"
	"github.com/callistaenterprise/goblog/common/config"
	"github.com/callistaenterprise/goblog/common/messaging"
	"github.com/spf13/viper"
	"os"
	"os/signal"
	"syscall"
)

var appName = "accountservice"

type PlainFormatter struct {
}

func (f PlainFormatter) Format(e *logrus.Entry) ([]byte, error) {
	var b *bytes.Buffer
	if e.Buffer != nil {
		b = e.Buffer
	} else {
		b = &bytes.Buffer{}
	}
	fmt.Fprintf(b, "%s", e.Message)
	b.WriteByte('\n')
	return b.Bytes(), nil
}
func init() {
	profile := flag.String("profile", "test", "Environment profile, something similar to spring profiles")
	configServerUrl := flag.String("configServerUrl", "http://configserver:8888", "Address to config server")
	configBranch := flag.String("configBranch", "master", "git branch to fetch configuration from")

	flag.Parse()

	viper.Set("profile", *profile)
	viper.Set("configServerUrl", *configServerUrl)
	viper.Set("configBranch", *configBranch)
}

func initLogger() {
	if viper.GetString("profile") != "dev" {
		//logrus.SetFormatter(&PlainFormatter{})
	}
}

func main() {
	logrus.SetFormatter(&logrus.JSONFormatter{})
	logrus.Info("Starting %v\n", appName)
	initLogger()
	config.LoadConfigurationFromBranch(
		viper.GetString("configServerUrl"),
		appName,
		viper.GetString("profile"),
		viper.GetString("configBranch"))
	//initializeBoltClient()

	service.DBClient = &dbclient.GormClient{}
	service.DBClient.SetupDB(viper.GetString("cockroachdb_conn_url"))
	service.DBClient.SeedAccounts()
	defer service.DBClient.Close()

	initializeMessaging()
	handleSigterm(func() {
		service.MessagingClient.Close()
	})
	service.StartWebServer(viper.GetString("server_port"))
}

func initializeMessaging() {
	if !viper.IsSet("amqp_server_url") {
		panic("No 'amqp_server_url' set in configuration, cannot start")
	}

	service.MessagingClient = &messaging.MessagingClient{}
	service.MessagingClient.ConnectToBroker(viper.GetString("amqp_server_url"))
	service.MessagingClient.Subscribe(viper.GetString("config_event_bus"), "topic", appName, config.HandleRefreshEvent)
}

func initializeBoltClient() {
	// service.DBClient = &dbclient.BoltClient{}
	// service.DBClient.OpenBoltDb()
	//  service.DBClient.Seed()
}

// Handles Ctrl+C or most other means of "controlled" shutdown gracefully. Invokes the supplied func before exiting.
func handleSigterm(handleExit func()) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	signal.Notify(c, syscall.SIGTERM)
	go func() {
		<-c
		handleExit()
		os.Exit(1)
	}()
}
