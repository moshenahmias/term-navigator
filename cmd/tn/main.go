package main

import (
	"context"

	tea "charm.land/bubbletea/v2"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/moshenahmias/term-navigator/internal/backends/local"
	s3exp "github.com/moshenahmias/term-navigator/internal/backends/s3"
	"github.com/moshenahmias/term-navigator/internal/ui"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
)

func newS3Client(ctx context.Context) (*s3.Client, error) {
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider("minioadmin", "minioadmin", ""),
		),
		config.WithRegion("us-east-1"),
		config.WithEndpointResolverWithOptions(
			aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
				return aws.Endpoint{
					URL:               "http://localhost:9000",
					HostnameImmutable: true,
				}, nil
			}),
		),
	)
	if err != nil {
		panic(err)
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = true // REQUIRED for MinIO
	})

	return client, nil
}

func main() {
	ctx := context.Background()
	left := local.NewExplorer(".")
	//right := local.NewExplorer(".")
	client, err := newS3Client(ctx)
	if err != nil {
		panic(err)
	}
	right := s3exp.NewExplorer(client, "moshe", "/")

	p := tea.NewProgram(ui.NewApp(ctx, left, right, 120, 30))

	if _, err := p.Run(); err != nil {
		panic(err)
	}

}
