package main

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go/logging"
	"github.com/moshenahmias/term-navigator/internal/backends/fakefs"
	"github.com/moshenahmias/term-navigator/internal/backends/local"
	s3exp "github.com/moshenahmias/term-navigator/internal/backends/s3"
	sftpexp "github.com/moshenahmias/term-navigator/internal/backends/sftp"

	appcfg "github.com/moshenahmias/term-navigator/internal/config"
	"github.com/moshenahmias/term-navigator/internal/file"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

var (
	factory = map[string]func(ctx context.Context, dev *appcfg.DeviceConfig) (map[string]file.Explorer, error){
		"local": func(ctx context.Context, dev *appcfg.DeviceConfig) (map[string]file.Explorer, error) {
			path := dev.Path
			if path == "" {
				path = "."
			}
			return map[string]file.Explorer{dev.Name: local.NewExplorer(path)}, nil
		},
		"fakefs": func(ctx context.Context, dev *appcfg.DeviceConfig) (map[string]file.Explorer, error) {
			return map[string]file.Explorer{dev.Name: fakefs.NewExplorer()}, nil
		},
		"s3": func(ctx context.Context, dev *appcfg.DeviceConfig) (map[string]file.Explorer, error) {
			// Build options dynamically
			opts := []func(*config.LoadOptions) error{}

			// Region override (optional)
			if dev.Region != "" {
				opts = append(opts, config.WithRegion(dev.Region))
			}

			// Credentials override (optional)
			if dev.Key != "" && dev.Secret != "" {
				opts = append(opts, config.WithCredentialsProvider(
					credentials.NewStaticCredentialsProvider(dev.Key, dev.Secret, dev.Session),
				))
			}

			opts = append(opts, config.WithClientLogMode(aws.ClientLogMode(0)), config.WithLogger(logging.Nop{}))

			// Load config (this will fall back to env/default chain if no overrides)
			cfg, err := config.LoadDefaultConfig(ctx, opts...)
			if err != nil {
				return nil, err
			}

			tlsConfig, err := NewTLSConfig(TLSConfigOptions{
				InsecureSkipVerify: dev.InsecureSkipVerify,
				CAFile:             dev.CAFile,
				ExpectedCertName:   dev.ExpectedCertName,
			})
			if err != nil {
				return nil, err
			}

			// Build S3 client
			client := s3.NewFromConfig(cfg, func(o *s3.Options) {
				if dev.Endpoint != "" {
					// MinIO / Localstack / custom S3-compatible
					o.BaseEndpoint = aws.String(dev.Endpoint)
				}

				o.UsePathStyle = true
				o.HTTPClient = &http.Client{
					Transport: &http.Transport{
						TLSClientConfig: tlsConfig,
					},
				}
			})

			if len(dev.Buckets) == 0 {
				out, err := client.ListBuckets(ctx, &s3.ListBucketsInput{})
				if err != nil {
					return nil, fmt.Errorf("listing buckets: %w", err)
				}

				for _, b := range out.Buckets {
					if b.Name != nil {
						dev.Buckets = append(dev.Buckets, *b.Name)
					}
				}
			}

			if len(dev.Buckets) == 0 {
				return nil, nil
			}

			explorers := make(map[string]file.Explorer, len(dev.Buckets))

			for _, bucket := range dev.Buckets {
				explorers[fmt.Sprintf("%s/%s", dev.Name, bucket)] = s3exp.NewExplorer(client, dev.Endpoint, dev.Region, bucket, "")
			}

			return explorers, nil
		},
		"sftp": func(ctx context.Context, dev *appcfg.DeviceConfig) (map[string]file.Explorer, error) {
			hostKeyCallback := ssh.InsecureIgnoreHostKey()

			if !dev.InsecureSkipVerify && dev.CAFile != "" {
				caData, err := os.ReadFile(dev.CAFile)
				if err != nil {
					return nil, fmt.Errorf("failed to read CA file: %w", err)
				}

				trustedKey, _, _, _, err := ssh.ParseAuthorizedKey(caData)
				if err != nil {
					return nil, fmt.Errorf("invalid CA public key: %w", err)
				}

				hostKeyCallback = func(hostname string, remote net.Addr, key ssh.PublicKey) error {
					if bytes.Equal(trustedKey.Marshal(), key.Marshal()) {
						return nil
					}
					return fmt.Errorf("host key mismatch for %s", hostname)
				}
			}

			cfg := &ssh.ClientConfig{
				User: dev.Key,
				Auth: []ssh.AuthMethod{
					ssh.Password(dev.Secret),
				},
				HostKeyCallback: hostKeyCallback,
			}

			conn, err := ssh.Dial("tcp", dev.Endpoint, cfg)
			if err != nil {
				return nil, err
			}
			client, err := sftp.NewClient(conn)
			if err != nil {
				return nil, err
			}

			path := dev.Path

			if path == "" {
				path = "/"
			}

			return map[string]file.Explorer{dev.Name: sftpexp.NewExplorer(client, dev.Endpoint, path)}, nil
		},
	}
)
