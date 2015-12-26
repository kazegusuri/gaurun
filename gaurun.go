package main

import (
	"./gaurun"
	"flag"
	"github.com/Sirupsen/logrus"
	statsGo "github.com/fukata/golang-stats-api-handler"
	"github.com/prometheus/client_golang/prometheus"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
)

func main() {
	versionPrinted := flag.Bool("v", false, "gaurun version")
	confPath := flag.String("c", "", "configuration file path for gaurun")
	flag.Parse()

	if *versionPrinted {
		gaurun.PrintGaurunVersion()
		os.Exit(0)
	}

	// set concurrency
	runtime.GOMAXPROCS(runtime.NumCPU())

	// set default parameters
	gaurun.ConfGaurun = gaurun.BuildDefaultConfGaurun()

	// init logger
	gaurun.LogAccess = logrus.New()
	gaurun.LogError = logrus.New()

	gaurun.LogAccess.Formatter = new(gaurun.GaurunFormatter)
	gaurun.LogError.Formatter = new(gaurun.GaurunFormatter)

	// load configuration
	conf, err := gaurun.LoadConfGaurun(gaurun.ConfGaurun, *confPath)
	if err != nil {
		gaurun.LogError.Fatal(err)
	}
	gaurun.ConfGaurun = conf

	// set logger
	err = gaurun.SetLogLevel(gaurun.LogAccess, "info")
	if err != nil {
		log.Fatal(err)
	}
	err = gaurun.SetLogLevel(gaurun.LogError, gaurun.ConfGaurun.Log.Level)
	if err != nil {
		log.Fatal(err)
	}
	err = gaurun.SetLogOut(gaurun.LogAccess, gaurun.ConfGaurun.Log.AccessLog)
	if err != nil {
		log.Fatal(err)
	}
	err = gaurun.SetLogOut(gaurun.LogError, gaurun.ConfGaurun.Log.ErrorLog)
	if err != nil {
		log.Fatal(err)
	}

	if !gaurun.ConfGaurun.Ios.Enabled && !gaurun.ConfGaurun.Android.Enabled {
		gaurun.LogError.Fatal("What do you want to do?")
	}

	if gaurun.ConfGaurun.Ios.Enabled {
		gaurun.CertificatePemIos.Cert, err = ioutil.ReadFile(gaurun.ConfGaurun.Ios.PemCertPath)
		if err != nil {
			gaurun.LogError.Fatal("A certification file for iOS is not found.")
		}

		gaurun.CertificatePemIos.Key, err = ioutil.ReadFile(gaurun.ConfGaurun.Ios.PemKeyPath)
		if err != nil {
			gaurun.LogError.Fatal("A key file for iOS is not found.")
		}

	}

	if gaurun.ConfGaurun.Android.Enabled {
		if gaurun.ConfGaurun.Android.ApiKey == "" {
			gaurun.LogError.Fatal("APIKey for Android is empty.")
		}
	}

	gaurun.ConfGaurun = conf
	gaurun.InitGCMClient()
	gaurun.InitStatGaurun()
	statsGo.PrettyPrintEnabled()
	gaurun.StartPushWorkers(gaurun.ConfGaurun.Core.WorkerNum, gaurun.ConfGaurun.Core.QueueNum)

	http.HandleFunc(gaurun.ConfGaurun.Api.PushUri, gaurun.PushNotificationHandler)
	http.HandleFunc(gaurun.ConfGaurun.Api.StatGoUri, statsGo.Handler)
	http.HandleFunc(gaurun.ConfGaurun.Api.StatAppUri, gaurun.StatsGaurunHandler)
	http.Handle(gaurun.ConfGaurun.Api.StatPrometheusUri, prometheus.Handler())
	http.HandleFunc(gaurun.ConfGaurun.Api.ConfigAppUri, gaurun.ConfigGaurunHandler)

	// Listen TCP Port
	if _, err := strconv.Atoi(gaurun.ConfGaurun.Core.Port); err == nil {
		http.ListenAndServe(":"+gaurun.ConfGaurun.Core.Port, nil)
	}

	// Listen UNIX Socket
	if strings.HasPrefix(gaurun.ConfGaurun.Core.Port, "unix:/") {
		sockPath := gaurun.ConfGaurun.Core.Port[5:]
		fi, err := os.Lstat(sockPath)
		if err == nil && (fi.Mode()&os.ModeSocket) == os.ModeSocket {
			err := os.Remove(sockPath)
			if err != nil {
				log.Fatal("failed to remove " + sockPath)
			}
		}
		l, err := net.Listen("unix", sockPath)
		if err != nil {
			log.Fatal("failed to listen: " + sockPath)
		}
		http.Serve(l, nil)
	}

	log.Fatal("port parameter is invalid: " + gaurun.ConfGaurun.Core.Port)
}
