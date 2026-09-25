package storage

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/lunebakami/holdotfiles-go/internal/backup"
)

func (r *R2) ArchiveKey() string { return strings.Trim(r.prefix, "/") + "/backup.zip" }

type BackupInfo struct {
	Computer string
	Modified time.Time
	Size     int64
}

func (r *R2) ListBackups(ctx context.Context) ([]string, error) {
	backups, err := r.ListBackupDetails(ctx)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(backups))
	for _, item := range backups {
		names = append(names, item.Computer)
	}
	sort.Strings(names)
	return names, nil
}

func (r *R2) ListBackupDetails(ctx context.Context) ([]BackupInfo, error) {
	client, ok := r.client.(*s3.Client)
	if !ok {
		return nil, fmt.Errorf("cliente não suporta listagem")
	}
	pager := s3.NewListObjectsV2Paginator(client, &s3.ListObjectsV2Input{Bucket: aws.String(r.bucket)})
	var backups []BackupInfo
	for pager.HasMorePages() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, object := range page.Contents {
			key := aws.ToString(object.Key)
			if strings.HasSuffix(key, "/backup.zip") {
				backups = append(backups, BackupInfo{
					Computer: strings.TrimSuffix(key, "/backup.zip"),
					Modified: aws.ToTime(object.LastModified),
					Size:     aws.ToInt64(object.Size),
				})
			}
		}
	}
	sort.Slice(backups, func(i, j int) bool {
		if backups[i].Modified.Equal(backups[j].Modified) {
			return backups[i].Computer < backups[j].Computer
		}
		return backups[i].Modified.After(backups[j].Modified)
	})
	return backups, nil
}

// Download returns a private temporary archive; caller must remove it.
func (r *R2) Download(ctx context.Context, computer string) (name string, err error) {
	if computer == "" || strings.HasPrefix(computer, "/") || strings.Contains(computer, "..") {
		return "", fmt.Errorf("computador inválido")
	}
	client, ok := r.client.(*s3.Client)
	if !ok {
		return "", fmt.Errorf("cliente não suporta download")
	}
	object, err := client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(r.bucket), Key: aws.String(computer + "/backup.zip")})
	if err != nil {
		return "", err
	}
	defer object.Body.Close()
	file, err := os.CreateTemp("", "holdotfiles-download-*.zip")
	if err != nil {
		return "", err
	}
	name = file.Name()
	defer func() {
		file.Close()
		if err != nil {
			os.Remove(name)
		}
	}()
	hash := sha256.New()
	n, err := io.Copy(io.MultiWriter(file, hash), io.LimitReader(object.Body, backup.MaxBytes+1))
	if err != nil {
		return name, err
	}
	if n > backup.MaxBytes {
		return name, fmt.Errorf("ZIP excede limite de 1 GiB")
	}
	if expected := object.Metadata["sha256"]; expected == "" || expected != fmt.Sprintf("%x", hash.Sum(nil)) {
		return name, fmt.Errorf("SHA-256 do ZIP ausente ou divergente")
	}
	err = file.Close()
	return name, err
}
