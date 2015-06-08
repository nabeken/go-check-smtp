package main

import (
	"fmt"
	"net/smtp"
	"os"
	"time"

	"github.com/fractalcat/nagiosplugin"
	"github.com/jessevdk/go-flags"
)

var opts struct {
	StartTLS bool          `short:"S" long:"starttls" description:"use STARTTLS" default:"false"`
	FQDN     string        `short:"F" long:"fqdn" description:"FQDN for HELO" default:"localhost"`
	Host     string        `short:"H" long:"hostname" description:"host name" required:"true"`
	Port     string        `short:"p" long:"port" description:"port number" default:"25"`
	Warning  time.Duration `short:"w" long:"warning" description:"response time to result in warning"`
	Critical time.Duration `short:"c" long:"critical" description:"response time to result in critical"`
}

func main() {
	if _, err := flags.Parse(&opts); err != nil {
		os.Exit(1)
	}
	check := nagiosplugin.NewCheck()
	defer check.Finish()

	start := time.Now()
	c, err := smtp.Dial(opts.Host + ":" + opts.Port)
	if err != nil {
		check.Criticalf("failed to connect to SMTP server: %s", err)
		return
	}
	d := time.Since(start)

	defer c.Quit()

	if err := c.Hello(opts.FQDN); err != nil {
		check.Criticalf("failed to say hello: %s", err)
		return
	}

	if opts.Critical > 0 && d.Nanoseconds() > opts.Critical.Nanoseconds() {
		check.Criticalf("SMTP: %s response time", d)
		return
	}

	check.AddResultf(nagiosplugin.OK, "SMTP: %s response time", d)

	fmt.Println("?")
}
