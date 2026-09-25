package storage

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
	"github.com/lunebakami/holdotfiles-go/internal/backup"
	appconfig "github.com/lunebakami/holdotfiles-go/internal/config"
)

type Result struct {
	Uploaded int
	Skipped  int
	Failed   int
}

type Syncer interface {
	Sync(context.Context, []string) (Result, error)
	Target() string
}

type objectClient interface {
	HeadObject(context.Context, *s3.HeadObjectInput, ...func(*s3.Options)) (*s3.HeadObjectOutput, error)
	PutObject(context.Context, *s3.PutObjectInput, ...func(*s3.Options)) (*s3.PutObjectOutput, error)
}

type R2 struct {
	client objectClient
	bucket string
	prefix string
	home   string
}

func NewR2(ctx context.Context, cfg appconfig.R2) (*R2, error) {
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AccessKeyID,
			cfg.SecretAccessKey,
			"",
		)),
		awsconfig.WithRegion("auto"),
	)
	if err != nil {
		return nil, fmt.Errorf("configurar cliente R2: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(options *s3.Options) {
		options.BaseEndpoint = aws.String(cfg.Endpoint)
		options.UsePathStyle = true
	})
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("obter diretório pessoal: %w", err)
	}
	return &R2{client: client, bucket: cfg.Bucket, prefix: cfg.Prefix, home: home}, nil
}

func (r *R2) Target() string {
	return fmt.Sprintf("r2://%s/%s", r.bucket, r.prefix)
}

func (r *R2) Sync(ctx context.Context, paths []string) (Result, error) {
	filename, err := backup.Pack(ctx, r.home, paths)
	if err != nil {
		return Result{Failed: 1}, err
	}
	defer os.Remove(filename)
	changed, err := r.upload(ctx, filename)
	if err != nil {
		return Result{Failed: 1}, err
	}
	if changed {
		return Result{Uploaded: 1}, nil
	}
	return Result{Skipped: 1}, nil
}

func (r *R2) upload(ctx context.Context, filename string) (bool, error) {
	file, err := os.Open(filename)
	if err != nil {
		return false, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return false, err
	}
	hash := sha256.New()
	if _, err := file.WriteTo(hash); err != nil {
		return false, fmt.Errorf("calcular hash: %w", err)
	}
	digest := fmt.Sprintf("%x", hash.Sum(nil))
	if _, err := file.Seek(0, 0); err != nil {
		return false, err
	}

	key := r.ArchiveKey()
	head, err := r.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(r.bucket),
		Key:    aws.String(key),
	})
	if err == nil && head.Metadata["sha256"] == digest {
		return false, nil
	}
	if err != nil && !isNotFound(err) {
		return false, fmt.Errorf("consultar objeto %q: %w", key, err)
	}

	contentType := mime.TypeByExtension(filepath.Ext(filename))
	if contentType == "" {
		buffer := make([]byte, 512)
		read, readErr := file.Read(buffer)
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			return false, readErr
		}
		contentType = http.DetectContentType(buffer[:read])
		if _, err := file.Seek(0, 0); err != nil {
			return false, err
		}
	}

	_, err = r.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(r.bucket),
		Key:           aws.String(key),
		Body:          file,
		ContentLength: aws.Int64(info.Size()),
		ContentType:   aws.String(contentType),
		Metadata:      map[string]string{"sha256": digest},
	})
	if err != nil {
		return false, fmt.Errorf("enviar objeto %q: %w", key, err)
	}
	return true, nil
}

func (r *R2) objectKey(filename string) string {
	clean := filepath.Clean(filename)
	relative, err := filepath.Rel(r.home, clean)
	if err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return joinKey(r.prefix, "home", filepath.ToSlash(relative))
	}
	volume := filepath.VolumeName(clean)
	clean = strings.TrimPrefix(clean, volume)
	clean = strings.TrimLeft(clean, `/\`)
	return joinKey(r.prefix, "absolute", filepath.ToSlash(clean))
}

func joinKey(parts ...string) string {
	return strings.Join(parts, "/")
}

func collectFiles(paths []string) ([]string, []error) {
	unique := make(map[string]struct{})
	var problems []error
	for _, root := range paths {
		info, err := os.Stat(root)
		if err != nil {
			problems = append(problems, fmt.Errorf("%s: %w", root, err))
			continue
		}
		if info.Mode().IsRegular() {
			unique[filepath.Clean(root)] = struct{}{}
			continue
		}
		if !info.IsDir() {
			problems = append(problems, fmt.Errorf("%s: tipo de arquivo não suportado", root))
			continue
		}

		err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				problems = append(problems, fmt.Errorf("%s: %w", path, walkErr))
				return nil
			}
			if entry.Type().IsRegular() {
				unique[filepath.Clean(path)] = struct{}{}
			} else if entry.Type()&os.ModeSymlink != 0 {
				target, statErr := os.Stat(path)
				if statErr != nil {
					problems = append(problems, fmt.Errorf("%s: %w", path, statErr))
				} else if target.Mode().IsRegular() {
					unique[filepath.Clean(path)] = struct{}{}
				}
			}
			return nil
		})
		if err != nil {
			problems = append(problems, fmt.Errorf("percorrer %s: %w", root, err))
		}
	}

	files := make([]string, 0, len(unique))
	for filename := range unique {
		files = append(files, filename)
	}
	sort.Strings(files)
	return files, problems
}

func isNotFound(err error) bool {
	var apiErr smithy.APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	return apiErr.ErrorCode() == "NotFound" || apiErr.ErrorCode() == "NoSuchKey"
}
