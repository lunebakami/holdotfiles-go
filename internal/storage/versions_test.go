package storage

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	appconfig "github.com/lunebakami/holdotfiles-go/internal/config"
)

func TestVersionListingAndDownloadSelection(t *testing.T) {
	const oldKey = "laptop/backup.zip"
	const newKey = "laptop/backup-2026-09-25T12-00-00.000000001Z.zip"
	var requested string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("list-type") == "2" {
			w.Header().Set("Content-Type", "application/xml")
			fmt.Fprintf(w, `<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><IsTruncated>false</IsTruncated><Contents><Key>%s</Key><LastModified>2026-09-24T12:00:00Z</LastModified><Size>3</Size></Contents><Contents><Key>%s</Key><LastModified>2026-09-25T12:00:00Z</LastModified><Size>3</Size></Contents><Contents><Key>laptop/unrelated.zip</Key><Size>1</Size></Contents></ListBucketResult>`, oldKey, newKey)
			return
		}
		requested = r.URL.Path
		body := []byte(requested)
		w.Header().Set("x-amz-meta-sha256", fmt.Sprintf("%x", sha256.Sum256(body)))
		w.Write(body)
	}))
	defer server.Close()
	remote, err := NewR2(context.Background(), appconfig.R2{Endpoint: server.URL, AccessKeyID: "test", SecretAccessKey: "test", Bucket: "bucket"})
	if err != nil {
		t.Fatal(err)
	}
	items, err := remote.ListBackupDetails(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].Key != newKey || items[1].Key != oldKey || items[0].Computer != "laptop" {
		t.Fatalf("unexpected versions: %+v", items)
	}
	for input, want := range map[string]string{oldKey: oldKey, newKey: newKey, "laptop": newKey} {
		filename, err := remote.Download(context.Background(), input)
		if err != nil {
			t.Fatal(err)
		}
		os.Remove(filename)
		if requested != "/bucket/"+want {
			t.Fatalf("download %s requested %s", input, requested)
		}
	}
}
