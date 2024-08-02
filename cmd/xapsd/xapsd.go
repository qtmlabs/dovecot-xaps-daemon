//
// The MIT License (MIT)
//
// Copyright (c) 2015 Stefan Arentz <stefan@arentz.ca>
// Copyright (c) 2017 Frederik Schwan <frederik dot schwan at linux dot com>
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.
//

package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"github.com/freswa/dovecot-xaps-daemon/internal"
	"github.com/freswa/dovecot-xaps-daemon/internal/config"
	"github.com/freswa/dovecot-xaps-daemon/internal/database"
	log "github.com/sirupsen/logrus"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

const Version = "1.1"

var configPath = flag.String("configPath", "", `Add an additional path to lookup the config file in`)
var configName = flag.String("configName", "", `Set a different configName (without extension) than the default "xapsd"`)
var generatePassword = flag.Bool("pass", false, `Generate a password hash to be used in the xapsd.yaml`)

func main() {
	flag.Parse()
	if *generatePassword {
		hashPassword()
	}
	config.ParseConfig(*configName, *configPath)
	cfg := config.GetOptions()
	lvl, err := log.ParseLevel(cfg.LogLevel)
	if err != nil {
		log.Fatal(err)
	}
	log.SetLevel(lvl)

	log.Debugln("Opening databasefile at", cfg.DatabaseFile)
	db, err := database.NewDatabase(cfg.DatabaseFile)
	if err != nil {
		log.Fatal("Cannot open databasefile: ", err)
	}

	apns := internal.NewApns(&cfg, db)

	var signalChan = make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	srv := internal.NewHttpSocket(&cfg, db, apns)

	sig := <-signalChan
	log.Infof("Stop signal received from system: %s, flushing database and shutting down...", sig)

	err = db.Write()

	if err != nil {
		log.Errorf("Failed to flush database: %v, ignoring.", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = srv.Shutdown(ctx)
	if err != nil {
		log.Errorln("Server shutdown failed: ", err)
	}
}

// function to generate the password
func hashPassword() {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Please enter the password -> ")
	text, _ := reader.ReadString('\n')
	// remove newlines
	text = strings.Replace(text, "\n", "", -1)
	hash := sha256.New()
	hash.Write([]byte(text))
	sha256sum := hex.EncodeToString(hash.Sum(nil))
	fmt.Printf("This is the hash -> %s\n", sha256sum)
	fmt.Print("For security reasons, we don't fill in the hash automagically. Please do so yourself.\n")
	os.Exit(0)
}
