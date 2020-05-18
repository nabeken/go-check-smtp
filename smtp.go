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

	CertWarn int `long:"cert-warn" description:"number of days a certificate has to be valid."`
	CertCrit int `long:"cert-crit" description:"number of days a certificate has to be valid."`

	ProxyProto bool `short:"P" long:"proxyproto" description:"use ProxyProtocol"`
	StartTLS   bool `short:"S" long:"starttls" description:"use STARTTLS"`

	// When the TLS verion is lower than this, the status will be warning.
	MinimumTLSVersion string `long:"minimum-tls-version" description:"minimum TLS version (e.g. 1.2)"`

	Timeout time.Duration `short:"t" long:"timeout" description:"connection times out" default:"10s"`

	Verbose bool `short:"v" long:"verbose" description:"verbose output for debugging"`
}

func main() {
	if _, err := flags.Parse(&opts); err != nil {
		if flagsErr, ok := err.(*flags.Error); ok && flagsErr.Type == flags.ErrHelp {
			os.Exit(0)
		} else {
			panic(err)
		}
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

	// Check a valid date in TLS certificate and the TLS version
	if opts.StartTLS && (opts.CertWarn > 0 || opts.CertCrit > 0) {
		if tlsState, ok := c.TLSConnectionState(); !ok {
			check.AddResultf(nagiosplugin.WARNING, "TLS state is not available")
		} else {
			if minTLSVer := opts.MinimumTLSVersion; minTLSVer != "" {
				tlsStatus := fmt.Sprintf(
					"%s >= %s (%s)",
					printTLSState(tlsState),
					minTLSVer,
					tls.CipherSuiteName(tlsState.CipherSuite),
				)

				v := convertToTLSVersion(minTLSVer)
				if tlsState.Version >= v {
					check.AddResult(nagiosplugin.OK, tlsStatus)
				} else {
					check.AddResult(nagiosplugin.WARNING, tlsStatus)
				}

				if opts.Verbose {
					fmt.Println(tlsStatus)
				}
			}

			now := time.Now()
			certStatus := "certificate '%s' expires in %d day(s) (%s)"
			for _, cert := range tlsState.PeerCertificates {
				warnNow := cert.NotAfter.AddDate(0, 0, -1*opts.CertWarn)
				critNow := cert.NotAfter.AddDate(0, 0, -1*opts.CertCrit)
				days := cert.NotAfter.Sub(now) / (24 * time.Hour)
				if now.After(warnNow) {
					check.AddResultf(nagiosplugin.WARNING, certStatus, cert.Subject.CommonName, days, cert.NotAfter)
				}
				if now.After(critNow) {
					check.AddResultf(nagiosplugin.CRITICAL, certStatus, cert.Subject.CommonName, days, cert.NotAfter)
				}
				check.AddResultf(nagiosplugin.OK, certStatus, cert.Subject.CommonName, days, cert.NotAfter)
			}
		}
	}
}

func convertToTLSVersion(ver string) uint16 {
	switch ver {
	case "1.0":
		return tls.VersionTLS10
	case "1.1":
		return tls.VersionTLS11
	case "1.2":
		return tls.VersionTLS12
	case "1.3":
		return tls.VersionTLS13
	default:
		panic("unknown TLS version")
	}
}

func printTLSState(tlsState tls.ConnectionState) string {
	switch tlsState.Version {
	case tls.VersionTLS10:
		return "TLS 1.0"
	case tls.VersionTLS11:
		return "TLS 1.1"
	case tls.VersionTLS12:
		return "TLS 1.2"
	case tls.VersionTLS13:
		return "TLS 1.3"
	default:
		return "unknown"
	}
}
