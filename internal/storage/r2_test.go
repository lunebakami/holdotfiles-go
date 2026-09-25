package storage

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

type storedObject struct {
	body     []byte
	metadata map[string]string
}

type memoryClient struct {
	objects map[string]storedObject
}

func newMemoryClient() *memoryClient {
	return &memoryClient{objects: make(map[string]storedObject)}
}

func (m *memoryClient) HeadObject(_ context.Context, input *s3.HeadObjectInput, _ ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
	object, ok := m.objects[aws.ToString(input.Key)]
	if !ok {
		return nil, &smithy.GenericAPIError{Code: "NotFound", Message: "ausente"}
	}
	return &s3.HeadObjectOutput{Metadata: object.metadata}, nil
}

func (m *memoryClient) PutObject(_ context.Context, input *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	body, err := io.ReadAll(input.Body)
	if err != nil {
		return nil, err
	}
	m.objects[aws.ToString(input.Key)] = storedObject{body: body, metadata: input.Metadata}
	return &s3.PutObjectOutput{}, nil
}

func TestSyncPreservesEveryVersion(t *testing.T) {
	home := t.TempDir()
	filename := filepath.Join(home, ".zshrc")
	if err := os.WriteFile(filename, []byte("export EDITOR=nvim\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	client := newMemoryClient()
	r2 := &R2{client: client, bucket: "dotfiles", prefix: "laptop", home: home}

	first, err := r2.Sync(context.Background(), []string{filename})
	if err != nil {
		t.Fatalf("primeira sincronização: %v", err)
	}
	if first.Uploaded != 1 || first.Skipped != 0 || first.Failed != 0 {
		t.Fatalf("resultado inesperado: %#v", first)
	}
	var object storedObject
	var firstKey string
	for key, value := range client.objects {
		firstKey, object = key, value
	}
	zr, err := zip.NewReader(bytes.NewReader(object.body), int64(len(object.body)))
	if err != nil {
		t.Fatal(err)
	}
	if len(zr.File) != 1 || zr.File[0].Name != ".zshrc" {
		t.Fatal("hierarquia ZIP incorreta")
	}
	in, err := zr.File[0].Open()
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(in)
	in.Close()
	if err != nil || !bytes.Equal(body, []byte("export EDITOR=nvim\n")) {
		t.Fatal("conteúdo ZIP incorreto")
	}

	second, err := r2.Sync(context.Background(), []string{filename})
	if err != nil {
		t.Fatalf("segunda sincronização: %v", err)
	}
	if second.Uploaded != 1 || second.Skipped != 0 || second.Failed != 0 {
		t.Fatalf("resultado inesperado: %#v", second)
	}
	if len(client.objects) != 2 || !bytes.Equal(client.objects[firstKey].body, object.body) {
		t.Fatal("versão anterior não preservada")
	}
}

func TestSyncReportsMissingPathAndContinues(t *testing.T) {
	home := t.TempDir()
	filename := filepath.Join(home, "exists")
	if err := os.WriteFile(filename, []byte("ok"), 0o600); err != nil {
		t.Fatal(err)
	}
	r2 := &R2{client: newMemoryClient(), bucket: "dotfiles", prefix: "host", home: home}

	result, err := r2.Sync(context.Background(), []string{filepath.Join(home, "missing"), filename})
	if err == nil {
		t.Fatal("era esperado erro parcial")
	}
	if result.Uploaded != 0 || result.Failed != 1 {
		t.Fatalf("resultado inesperado: %#v", result)
	}
}

func TestCollectFilesIsRecursiveSortedAndUnique(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "nested")
	if err := os.Mkdir(nested, 0o700); err != nil {
		t.Fatal(err)
	}
	first := filepath.Join(root, "a")
	second := filepath.Join(nested, "b")
	for _, filename := range []string{first, second} {
		if err := os.WriteFile(filename, []byte(filename), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	got, problems := collectFiles([]string{root, first})
	if len(problems) != 0 {
		t.Fatalf("problemas inesperados: %v", problems)
	}
	want := []string{first, second}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("collectFiles = %#v; esperado %#v", got, want)
	}
}
