package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
)

// TLSConfigOptions lets you customize how the TLS config is built.
type TLSConfigOptions struct {
	InsecureSkipVerify bool   // allow self‑signed / invalid certs
	CAFile             string // optional path to custom CA bundle
	ExpectedCertName   string // override SNI if needed
}

// NewTLSConfig builds a tls.Config based on the provided options.
func NewTLSConfig(opts TLSConfigOptions) (*tls.Config, error) {
	cfg := &tls.Config{
		InsecureSkipVerify: opts.InsecureSkipVerify,
		ServerName:         opts.ExpectedCertName,
	}

	// Load custom CA if provided
	if opts.CAFile != "" {
		caCert, err := os.ReadFile(opts.CAFile)
		if err != nil {
			return nil, err
		}

		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to parse CA file: %s", opts.CAFile)
		}

		cfg.RootCAs = pool
	}

	return cfg, nil
}
