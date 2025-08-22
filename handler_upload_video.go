package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/database"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/utils"
	"github.com/google/uuid"
)

var allowVideoTypes = [...]string{
	"video/mp4",
}

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	// Set upload limit
	r.Body = http.MaxBytesReader(w, r.Body, 1<<30)

	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	fmt.Println("uploading video", videoID, "by user", userID)

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to get video", err)
		return
	}

	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized", err)
		return
	}

	file, header, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}
	defer file.Close()

	mediaType, _, err := mime.ParseMediaType(header.Header.Get("Content-Type"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse Content-Type", err)
		return
	}

	// Media type check
	if n := len(allowVideoTypes); n > 0 {
		for i := 0; i <= n; i++ {
			if i == n {
				respondWithError(w, http.StatusBadRequest, fmt.Sprintf("Cannot upload %s file", mediaType), err)
				return
			}

			if mediaType == allowVideoTypes[i] {
				break
			}
		}
	}

	key := make([]byte, 32)
	rand.Read(key)
	fileName := base64.RawURLEncoding.EncodeToString(key)

	fileExtension := strings.Split(mediaType, "/")[1]
	filePath := filepath.Join(fmt.Sprintf("%s.%s", fileName, fileExtension))
	dst, err := os.CreateTemp("", filePath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to create file", err)
		return
	}

	defer os.Remove(filePath)
	defer dst.Close()

	_, err = io.Copy(dst, file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to upload video", err)
		return
	}

	_, err = dst.Seek(0, io.SeekStart)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to read temp video", err)
		return
	}

	// Get video aspect ratio
	ratio, err := utils.GetVideoAspectRatio(dst.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to get video aspect ratio", err)
		return
	}

	prefix := ""
	switch ratio {
	case "16:9":
		prefix = "landscape"
	case "9:16":
		prefix = "portrait"
	default:
		prefix = "other"
	}

	// Process video
	tempFilePath, err := utils.ProcessVideoForFastStart(dst.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to process video", err)
		return
	}
	defer os.Remove(tempFilePath)

	tempFile, _ := os.Open(tempFilePath)

	// Upload to s3
	fileKey := fmt.Sprintf("%s/%s", prefix, filePath)
	_, err = cfg.s3Client.PutObject(r.Context(), &s3.PutObjectInput{
		Bucket:      &cfg.s3Bucket,
		Key:         &fileKey,
		Body:        tempFile,
		ContentType: &mediaType,
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to upload video to s3", err)
		return
	}

	// Update video URL
	videoURL := fmt.Sprintf("%s,%s", cfg.s3Bucket, fileKey)
	video.VideoURL = &videoURL
	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to update video", err)
		return
	}

	video, err = cfg.dbVideoToSignedVideo(video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to get signed video", err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}

func generatePresignedURL(s3Client *s3.Client, bucket, key string, expireTime time.Duration) (string, error) {
	presignClient := s3.NewPresignClient(s3Client)
	request, err := presignClient.PresignGetObject(context.Background(), &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	}, s3.WithPresignExpires(expireTime))
	if err != nil {
		return "", err
	}

	return request.URL, nil
}

func (cfg *apiConfig) dbVideoToSignedVideo(video database.Video) (database.Video, error) {
	if video.VideoURL == nil {
		return video, nil
	}

	videoURL := *video.VideoURL
	bucket, key, found := strings.Cut(videoURL, ",")
	if !found {
		return video, fmt.Errorf("failed to get bucket and key from video URL: %s", videoURL)
	}

	presignedURL, err := generatePresignedURL(cfg.s3Client, bucket, key, time.Minute)
	if err != nil {
		return video, err
	}

	video.VideoURL = &presignedURL

	return video, nil
}
