package main

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"os"
	"time"

	"github.com/jessevdk/go-flags"
	"github.com/nabeken/nagiosplugin"
)

var tlsConfig = &tls.Config{
	InsecureSkipVerify: true,
}

var opts struct {
	FQDN string `short:"F" long:"fqdn" description:"FQDN for HELO" default:"localhost"`
	Host string `short:"H" long:"hostname" description:"host name" required:"true"`
	Port string `short:"p" long:"port" description:"port number" default:"25"`

	MailFrom string `short:"f" long:"from" description:"sender"`
	RcptTo   string `short:"r" long:"recipient" description:"recipient"`

	Warning  time.Duration `short:"w" long:"warning" description:"response time to result in warning"`
	Critical time.Duration `short:"c" long:"critical" description:"response time to result in critical"`

	ProxyProto bool `short:"P" long:"proxyproto" description:"use ProxyProtocol" default:"false"`
	StartTLS   bool `short:"S" long:"starttls" description:"use STARTTLS" default:"false"`

	Timeout time.Duration `short:"t" long:"timeout" description:"connection times out" default:"10s"`

	Verbose bool `short:"v" long:"verbose" description:"verbose output for debugging"`
}

func main() {
	if _, err := flags.Parse(&opts); err != nil {
		os.Exit(1)
	}

	check := nagiosplugin.NewCheck("SMTP")
	defer check.Finish()

	start := time.Now()
	conn, err := net.DialTimeout("tcp", opts.Host+":"+opts.Port, opts.Timeout)
	if err != nil {
		check.Criticalf("failed to connect to SMTP server: %s", err)
	}

	if opts.Verbose {
		fmt.Println(conn.LocalAddr(), conn.RemoteAddr())
	}

	if opts.ProxyProto {
		conn = &ProxyConn{Conn: conn}
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(opts.Timeout))

	c, err := smtp.NewClient(conn, "")
	if err != nil {
		check.Criticalf("failed to connect to SMTP server: %s", err)
	}
	defer c.Quit()

	if err := c.Hello(opts.FQDN); err != nil {
		check.Criticalf("failed to say hello: %s", err)
	}

	if opts.StartTLS {
		if err := c.StartTLS(tlsConfig); err != nil {
			check.Criticalf("failed to establish TLS connection: %s", err)
		}
	}

	// measure response time including STARTTLS procedure
	d := time.Since(start)
	if opts.Warning > 0 && d.Nanoseconds() > opts.Warning.Nanoseconds() {
		check.AddResultf(nagiosplugin.WARNING, "%s response time", d)
	}
	if opts.Critical > 0 && d.Nanoseconds() > opts.Critical.Nanoseconds() {
		check.AddResultf(nagiosplugin.CRITICAL, "%s response time", d)
	}

	check.AddResultf(nagiosplugin.OK, "%s response time", d)

	if f := opts.MailFrom; f != "" {
		if err := c.Mail(f); err != nil {
			check.Criticalf("MAIL command was not accepted: %s", err)
		}
	}

	if r := opts.RcptTo; r != "" {
		if err := c.Rcpt(r); err != nil {
			check.Criticalf("RCPT command was not accepted: %s", err)
		}
	}
}
