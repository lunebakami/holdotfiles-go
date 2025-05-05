package lib

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/appwrite/sdk-for-go/appwrite"
	"github.com/appwrite/sdk-for-go/client"
	"github.com/appwrite/sdk-for-go/models"
	"github.com/joho/godotenv"
)

type AppwriteClient struct {
	Client *client.Client
	Bucket *models.Bucket
}

func (a *AppwriteClient) Sync(paths []string) {
	for _, path := range paths {
		if path != "" {
			filepath, err := ExpandPath(path)
			if err != nil {
				fmt.Println(err)
			}
			// TODO: Upload file to appwrite
		}
	}
}

var ErrStorageBucketNotFound = errors.New("Storage bucket with the requested ID could not be found.")

func getBucket(c *client.Client) (*models.Bucket, error) {
	bucketID := os.Getenv("APPWRITE_BUCKET_ID")
	bucketName := os.Getenv("APPWRITE_BUCKET_NAME")

	bucket, err := appwrite.NewStorage(*c).GetBucket(bucketID)
	if err != nil {
		if errors.Is(err, ErrStorageBucketNotFound) ||
			strings.Contains(err.Error(), "bucket with the requested ID could not be found") {
			return appwrite.NewStorage(*c).CreateBucket(bucketID, bucketName)
		}
		return nil, fmt.Errorf("failed to get bucket: %w", err)
	}

	return bucket, nil
}

func InitAppwrite() (*AppwriteClient, error) {
	err := godotenv.Load()
	if err != nil {
		log.Printf("Error loading .env file: %v", err)
		return nil, fmt.Errorf("failed to load .env file: %w", err)
	}

	appwriteClient := appwrite.NewClient(
		appwrite.WithEndpoint(os.Getenv("APPWRITE_ENDPOINT")),
		appwrite.WithProject(os.Getenv("APPWRITE_PROJECT_ID")),
		appwrite.WithKey(os.Getenv("APPWRITE_KEY")),
	)

	appwriteBucket, err := getBucket(&appwriteClient)
	if err != nil {
		return nil, fmt.Errorf("failed to get bucket: %w", err)
	}

	return &AppwriteClient{
		Client: &appwriteClient,
		Bucket: appwriteBucket,
	}, nil
}
