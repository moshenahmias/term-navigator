package main

import (
	"context"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/moshenahmias/term-navigator/internal/backends/fakefs"
	"github.com/moshenahmias/term-navigator/internal/backends/local"
	s3exp "github.com/moshenahmias/term-navigator/internal/backends/s3"
	appcfg "github.com/moshenahmias/term-navigator/internal/config"
	"github.com/moshenahmias/term-navigator/internal/file"
)

var (
	factory = map[string]func(ctx context.Context, dev *appcfg.DeviceConfig) (file.Explorer, error){
		"local": func(ctx context.Context, dev *appcfg.DeviceConfig) (file.Explorer, error) {
			path := dev.Path
			if path == "" {
				path = "."
			}
			return local.NewExplorer(path), nil
		},
		"fakefs": func(ctx context.Context, dev *appcfg.DeviceConfig) (file.Explorer, error) {
			return fakefs.NewExplorer(), nil
		},
		"s3": func(ctx context.Context, dev *appcfg.DeviceConfig) (file.Explorer, error) {
			// Build options dynamically
			opts := []func(*config.LoadOptions) error{}

			// Region override (optional)
			if dev.Region != "" {
				opts = append(opts, config.WithRegion(dev.Region))
			}

			// Credentials override (optional)
			if dev.Key != "" && dev.Secret != "" {
				opts = append(opts, config.WithCredentialsProvider(
					credentials.NewStaticCredentialsProvider(dev.Key, dev.Secret, ""),
				))
			}

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
					o.UsePathStyle = true
					o.HTTPClient = &http.Client{
						Transport: &http.Transport{
							TLSClientConfig: tlsConfig,
						},
					}
				}
			})

			return s3exp.NewExplorer(client, dev.Endpoint, dev.Region, dev.Bucket, dev.Prefix), nil
		},
	}
)
