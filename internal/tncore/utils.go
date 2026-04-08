package tncore

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"os"
	"strings"
)

func bytesFormatter(n int64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
		TB = 1024 * GB
	)

	switch {
	case n < KB:
		return fmt.Sprintf("%d B", n)
	case n < MB:
		return fmt.Sprintf("%.1f KB", float64(n)/KB)
	case n < GB:
		return fmt.Sprintf("%.1f MB", float64(n)/MB)
	case n < TB:
		return fmt.Sprintf("%.2f GB", float64(n)/GB)
	default:
		return fmt.Sprintf("%.2f TB", float64(n)/TB)
	}
}

func listTarGz(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)

	var sb strings.Builder

	for {
		hdr, err := tr.Next()
		if err != nil {
			break
		}
		sb.WriteString(hdr.Name)
		sb.WriteString("\n")
	}

	return sb.String(), nil
}

func listZip(path string) (string, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return "", err
	}
	defer r.Close()

	var sb strings.Builder

	for _, f := range r.File {
		sb.WriteString(f.Name)
		sb.WriteString("\n")
	}
	return sb.String(), nil
}
