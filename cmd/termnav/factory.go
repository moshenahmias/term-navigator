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
	s3exp "github.com/moshenahmias/term-navigator/internal/backends/s3"
	sftpexp "github.com/moshenahmias/term-navigator/internal/backends/sftp"

	appcfg "github.com/moshenahmias/term-navigator/internal/config"
	"github.com/moshenahmias/term-navigator/internal/file"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

func buildConstructors(cfg *appcfg.Config) (map[string]*file.LazyDevice, error) {
	result := make(map[string]*file.LazyDevice)

	for i, devCfg := range cfg.Devices {
		if devCfg.Disabled && !loadDisabledFlag {
			continue
		}

		if devCfg.Type == "" {
			return nil, fmt.Errorf("device %d (%s) missing type", i, devCfg.Name)
		}

		constructor, ok := constructors[devCfg.Type]
		if !ok {
			return nil, fmt.Errorf("unknown device type: %s", devCfg.Type)
		}

		result[devCfg.Name] = file.NewLazyDevice(constructor(&devCfg))
	}

	return result, nil
}

var constructors = map[string]func(dev *appcfg.DeviceConfig) file.ExplorerConstructor{
	"fakefs": func(dev *appcfg.DeviceConfig) file.ExplorerConstructor {
		return func(ctx context.Context) (map[string]file.Explorer, error) {
			return map[string]file.Explorer{dev.Name: fakefs.NewExplorer()}, nil
		}
	},
	"s3": func(dev *appcfg.DeviceConfig) file.ExplorerConstructor {
		return func(ctx context.Context) (map[string]file.Explorer, error) {
			opts := []func(*config.LoadOptions) error{}

			if dev.Region != "" {
				opts = append(opts, config.WithRegion(dev.Region))
			}

			if dev.Key != "" && dev.Secret != "" {
				opts = append(opts, config.WithCredentialsProvider(
					credentials.NewStaticCredentialsProvider(dev.Key, dev.Secret, dev.Session),
				))
			}

			opts = append(opts, config.WithClientLogMode(aws.ClientLogMode(0)), config.WithLogger(logging.Nop{}))

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

			client := s3.NewFromConfig(cfg, func(o *s3.Options) {
				if dev.Endpoint != "" {
					o.BaseEndpoint = aws.String(dev.Endpoint)
				}

				o.UsePathStyle = true
				o.HTTPClient = &http.Client{
					Transport: &http.Transport{
						TLSClientConfig: tlsConfig,
					},
				}
			})

			buckets := dev.Buckets
			if len(buckets) == 0 {
				out, err := client.ListBuckets(ctx, &s3.ListBucketsInput{})
				if err != nil {
					return nil, fmt.Errorf("listing buckets: %w", err)
				}

				for _, b := range out.Buckets {
					if b.Name != nil {
						buckets = append(buckets, *b.Name)
					}
				}
			}

			if len(buckets) == 0 {
				return nil, fmt.Errorf("no buckets found or specified")
			}

			explorers := make(map[string]file.Explorer, len(buckets))
			for _, bucket := range buckets {
				explorers[fmt.Sprintf("%s/%s", dev.Name, bucket)] = s3exp.NewExplorer(client, dev.Endpoint, dev.Region, bucket, "")
			}

			return explorers, nil
		}
	},
	"sftp": func(dev *appcfg.DeviceConfig) file.ExplorerConstructor {
		return func(ctx context.Context) (map[string]file.Explorer, error) {
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
		}
	},
}
